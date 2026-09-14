package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ── Configuration ──────────────────────────────────────────────────────────────

// Person is one payer. Order matters: it decides column order in JSON arrays
// and CSV exports (Python relied on dict insertion order for this).
type Person struct {
	Name  string
	Ratio float64
}

var SplitRatio = []Person{
	{Name: "Andreas", Ratio: 0.60},
	{Name: "Nikoline", Ratio: 0.40},
}

// Cardholders maps a person to the substrings that may appear in the CSV's
// "Kortmedlem" column (which holds the full name, e.g. OLA NORDMANN).
var Cardholders = []struct {
	Name    string
	Matches []string
}{
	{Name: "Andreas", Matches: []string{"ANDREAS"}},
	{Name: "Nikoline", Matches: []string{"NIKOLINE"}},
}

// SkipNonPositive drops rows with an amount <= 0, matching the old PDF parser.
// Amex writes credits/refunds with a leading minus; note that the sample export
// uses the Unicode minus U+2212, not an ASCII hyphen. Set to false to keep them.
const SkipNonPositive = true

var categoryRuleSrc = [][2]string{
	{`kiwi|meny|coop|joker|rema|bunnpris|extra`, "Groceries"},
	{`wolt|foodora|just ?eat|mcdonald|burger|pizza|sushi`, "Takeaway"},
	{`kaffebrenneriet|joe.the.juice|starbucks|espresso`, "Café"},
	{`restaurant|bistro|grill|mat og drikke|ochaya`, "Dining out"},
	{`ruterappen|ruter|atb|skyss|kolumbus|entur`, "Public transport"},
	{`bolt|uber|taxi|lyft|cabonline`, "Taxi"},
	{`wideroe|sas|norwegian|flyr|ryanair|lufthansa`, "Flights"},
	{`h.m|zara|mango|weekday|cos |arket|monki|nakd`, "Clothing"},
	{`elkjop|komplett|power |apple|samsung|kjell`, "Electronics"},
	{`clas ohlson|biltema|byggmax|jula`, "Hardware / tools"},
	{`nordicnest|ikea|jysk|bolia|hay |diy`, "Home & interior"},
	{`skolyx|zalando|boozt|footlocker`, "Shoes"},
	{`nikita|caia|lookfantastic|kicks`, "Beauty & personal care"},
	{`ticketmaster|billettservice|eventbrite`, "Events / tickets"},
	{`colosseum kino|sf kino|nordisk film kino`, "Cinema"},
	{`spotify|netflix|youtube|apple.com.bill|hbo|viaplay|disney`, "Streaming"},
	{`vinmonopolet`, "Alcohol"},
	{`jagex|steam|playstation|xbox|nintendo|kodekloud`, "Games / learning"},
	{`claude\.ai|anthropic|openai|chatgpt`, "AI subscriptions"},
	{`hetzner|digitalocean|aws|google cloud|azure|linode`, "Cloud / hosting"},
	{`linuxfoundation|udemy|coursera|pluralsight`, "Education"},
	{`paypal \*google|google\.com`, "Google services"},
	{`wikipedia`, "Donations"},
	{`apotek|vitusapotek|boots|farma`, "Pharmacy"},
	{`gym|trening|crossfit|sats|evo fitness`, "Fitness"},
	{`havferd`, "Activities / experiences"},
	{`rouleur|markedsplassen|oslo gate|lett `, "Other shopping"},
	{`kunstnernes`, "Culture"},
	{`medlemsavgift`, "Card membership fee"},
}

var SharedCategories = map[string]bool{
	"Groceries":                true,
	"Takeaway":                 true,
	"Dining out":               true,
	"Alcohol":                  true,
	"Activities / experiences": true,
	"Café":                     true,
	"Card membership fee":      true,
}

type categoryRule struct {
	re  *regexp.Regexp
	cat string
}

var (
	categoryRules     []categoryRule
	builtinCategories []string
	multiSpace        = regexp.MustCompile(`\s+`)
)

func init() {
	seen := map[string]bool{"Uncategorized": true}
	for _, r := range categoryRuleSrc {
		categoryRules = append(categoryRules, categoryRule{
			re:  regexp.MustCompile(r[0]),
			cat: r[1],
		})
		seen[r[1]] = true
	}
	for cat := range seen {
		builtinCategories = append(builtinCategories, cat)
	}
	sort.Strings(builtinCategories)
}

// PersonNames returns the payers in configured order.
func PersonNames() []string {
	names := make([]string, 0, len(SplitRatio))
	for _, p := range SplitRatio {
		names = append(names, p.Name)
	}
	return names
}

// CardholderNames returns the configured cardholders in order.
func CardholderNames() []string {
	out := make([]string, 0, len(Cardholders))
	for _, c := range Cardholders {
		out = append(out, c.Name)
	}
	return out
}

// ── Parsed transaction ─────────────────────────────────────────────────────────

type ParsedTx struct {
	Date        string // dd.mm.yy, same format the PDF parser produced
	Description string
	Amount      float64
	Cardholder  string
	Category    string
	IsShared    bool
	City        string
	Country     string

	when time.Time // used for sorting only, not persisted
}

// ── CSV parsing ────────────────────────────────────────────────────────────────

var dateLayouts = []string{
	"01/02/2006", // Amex export format (MM/DD/YYYY)
	"01/02/06",
	"2006-01-02",
	"02.01.2006",
	"02.01.06",
}

// ParseCSV reads an Amex CSV export and returns the transactions it contains.
func ParseCSV(data []byte) ([]ParsedTx, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // Excel loves a BOM

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = sniffDelimiter(data)
	r.FieldsPerRecord = -1 // rows have trailing empty fields
	r.LazyQuotes = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read CSV: %w", err)
	}
	if len(rows) < 2 {
		return nil, errors.New("CSV contains no transactions")
	}

	head := rows[0]
	iDate := columnIndex(head, "dato", "date")
	iDesc := columnIndex(head, "beskrivelse", "description")
	iAmount := columnIndex(head, "beløp", "belop", "amount")
	iMember := columnIndex(head, "kortmedlem", "kortinnehaver", "card member")
	iCity := columnIndex(head, "sted", "city")
	iCountry := columnIndex(head, "land", "country")

	if iDate < 0 || iDesc < 0 || iAmount < 0 {
		return nil, errors.New(`CSV is missing one of the required columns: "Dato", "Beskrivelse", "Beløp"`)
	}

	defaultHolder := Cardholders[0].Name
	out := make([]ParsedTx, 0, len(rows)-1)

	for _, row := range rows[1:] {
		raw := field(row, iAmount)
		if raw == "" {
			continue
		}
		amount, err := parseNOK(raw)
		if err != nil {
			continue
		}
		if SkipNonPositive && amount <= 0 {
			continue
		}

		when, err := parseDate(field(row, iDate))
		if err != nil {
			continue
		}

		desc := collapseSpaces(field(row, iDesc))
		if desc == "" {
			continue
		}

		holder := matchCardholder(field(row, iMember), defaultHolder)
		cat, shared := classify(desc)

		out = append(out, ParsedTx{
			Date:        when.Format("02.01.06"),
			Description: desc,
			Amount:      amount,
			Cardholder:  holder,
			Category:    cat,
			IsShared:    shared,
			City:        collapseSpaces(field(row, iCity)),
			Country:     collapseSpaces(field(row, iCountry)),
			when:        when,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].when.Equal(out[j].when) {
			return out[i].when.Before(out[j].when)
		}
		return out[i].Cardholder < out[j].Cardholder
	})

	return out, nil
}

// classify returns the category for a description and whether it should be
// split. Saved rules are checked first, so a correction beats the defaults
// compiled in below; a rule may also override the split directly, which is how
// "Vinmonopolet is shared but Steam is not" gets expressed without inventing a
// category for each.
func classify(description string) (string, bool) {
	desc := strings.ToLower(description)

	for _, r := range currentRules() {
		if r.re.MatchString(desc) {
			if r.IsShared != nil {
				return r.Category, *r.IsShared
			}
			return r.Category, sharedDefault(r.Category)
		}
	}

	for _, rule := range categoryRules {
		if rule.re.MatchString(desc) {
			return rule.cat, sharedDefault(rule.cat)
		}
	}
	return "Uncategorized", sharedDefault("Uncategorized")
}

// ── Small helpers ──────────────────────────────────────────────────────────────

func sniffDelimiter(data []byte) rune {
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}
	if bytes.Count(line, []byte{';'}) > bytes.Count(line, []byte{','}) {
		return ';'
	}
	return ','
}

func columnIndex(header []string, names ...string) int {
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(strings.Trim(h, `"`)))
		for _, n := range names {
			if h == n {
				return i
			}
		}
	}
	return -1
}

func field(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func collapseSpaces(s string) string {
	return strings.TrimSpace(multiSpace.ReplaceAllString(s, " "))
}

var amountCleaner = strings.NewReplacer(
	"\u2212", "-", // Unicode minus, used by the Amex export
	"\u2013", "-", // en dash, just in case
	"\u00a0", "", // non-breaking space
	" ", "",
	"'", "",
	"kr", "",
	"NOK", "",
)

func parseNOK(s string) (float64, error) {
	s = amountCleaner.Replace(strings.TrimSpace(s))
	if strings.Contains(s, ",") {
		// Norwegian format: dot is a thousands separator, comma is the decimal.
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	if s == "" {
		return 0, errors.New("empty amount")
	}
	return strconv.ParseFloat(s, 64)
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q", s)
}

func matchCardholder(member, fallback string) string {
	m := strings.ToUpper(member)
	for _, c := range Cardholders {
		for _, want := range c.Matches {
			if strings.Contains(m, strings.ToUpper(want)) {
				return c.Name
			}
		}
	}
	return fallback
}
