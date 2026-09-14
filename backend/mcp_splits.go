package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deciding whether a charge was joint is a judgement call that a category
// cannot make: groceries bought on a solo work trip are not shared, and a
// 15,000 NOK sofa filed under Electronics is. What a category can do is set a
// sensible default; these tools are for finding and fixing the places where the
// default is wrong.
//
// The ordering principle is money, not count. Getting the split wrong on a
// 42 NOK coffee moves 15 NOK; getting it wrong on a flight moves thousands. So
// review_splits ranks by how much the balance would move if the flag flipped,
// which puts the handful of rows worth a human's attention at the top.

// ── review_splits ──────────────────────────────────────────────────────────────

type reviewArgs struct {
	InvoiceID     int64   `json:"invoice_id,omitempty" jsonschema:"limit to one statement; omit or 0 for all"`
	MinImpact     float64 `json:"min_impact,omitempty" jsonschema:"only show rows where flipping the split moves at least this many NOK; default 100"`
	IncludeLocked bool    `json:"include_locked,omitempty" jsonschema:"also show splits that were already decided deliberately; default false"`
	Limit         int     `json:"limit,omitempty" jsonschema:"maximum rows to return, default 25, max 100"`
}

type splitCandidate struct {
	TransactionID   int64   `json:"transaction_id"`
	Date            string  `json:"date"`
	Description     string  `json:"description"`
	Amount          float64 `json:"amount"`
	Cardholder      string  `json:"cardholder"`
	Category        string  `json:"category"`
	IsShared        bool    `json:"is_shared"`
	Impact          float64 `json:"impact"`
	Locked          bool    `json:"split_locked"`
	Country         string  `json:"country,omitempty"`
	City            string  `json:"city,omitempty"`
	Abroad          bool    `json:"abroad"`
	PartnerSameDay  int     `json:"partner_transactions_same_day"`
	PartnerSameTrip bool    `json:"partner_abroad_same_day"`
}

type reviewOut struct {
	Candidates  []splitCandidate `json:"candidates"`
	TotalImpact float64          `json:"total_impact"`
	Reviewed    int              `json:"rows_considered"`
	Currency    string           `json:"currency"`
}

func (t *mcpTools) reviewSplits(ctx context.Context, req *mcp.CallToolRequest, in reviewArgs) (
	*mcp.CallToolResult, reviewOut, error,
) {
	minImpact := in.MinImpact
	if minImpact <= 0 {
		minImpact = 100
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	q := `SELECT ` + txColumns + ` FROM "transaction"`
	var args []any
	var conds []string
	if in.InvoiceID > 0 {
		args = append(args, in.InvoiceID)
		conds = append(conds, fmt.Sprintf(`invoice_id = $%d`, len(args)))
	}
	if !in.IncludeLocked {
		conds = append(conds, `split_locked = false`)
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}

	rows, err := t.db.Query(q, args...)
	if err != nil {
		return nil, reviewOut{}, err
	}
	defer rows.Close()

	var all []Transaction
	for rows.Next() {
		tx, err := scanTransaction(rows)
		if err != nil {
			return nil, reviewOut{}, err
		}
		all = append(all, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, reviewOut{}, err
	}

	// Context signals, computed once over the whole set: was the other person
	// spending on the same day, and was either of them abroad.
	perDay := map[string]map[string]int{}     // date -> cardholder -> count
	abroadDay := map[string]map[string]bool{} // date -> cardholder -> abroad
	for _, tx := range all {
		if perDay[tx.Date] == nil {
			perDay[tx.Date] = map[string]int{}
			abroadDay[tx.Date] = map[string]bool{}
		}
		perDay[tx.Date][tx.Cardholder]++
		if isAbroad(tx.Country) {
			abroadDay[tx.Date][tx.Cardholder] = true
		}
	}

	ratios := map[string]float64{}
	for _, p := range SplitRatio {
		ratios[p.Name] = p.Ratio
	}

	out := reviewOut{Currency: "NOK", Candidates: []splitCandidate{}, Reviewed: len(all)}

	for _, tx := range all {
		// How much the balance moves if this flag flips, in either direction.
		impact := round2(tx.Amount * (1 - ratios[tx.Cardholder]))
		if impact < minImpact {
			continue
		}

		partnerCount := 0
		partnerAbroad := false
		for _, p := range SplitRatio {
			if p.Name == tx.Cardholder {
				continue
			}
			partnerCount += perDay[tx.Date][p.Name]
			if abroadDay[tx.Date][p.Name] {
				partnerAbroad = true
			}
		}

		out.Candidates = append(out.Candidates, splitCandidate{
			TransactionID: tx.ID, Date: tx.Date, Description: tx.Description,
			Amount: tx.Amount, Cardholder: tx.Cardholder, Category: tx.Category,
			IsShared: tx.IsShared, Impact: impact, Locked: tx.SplitLocked,
			Country: tx.Country, City: tx.City, Abroad: isAbroad(tx.Country),
			PartnerSameDay: partnerCount, PartnerSameTrip: partnerAbroad,
		})
	}

	sort.Slice(out.Candidates, func(i, j int) bool {
		return out.Candidates[i].Impact > out.Candidates[j].Impact
	})
	if len(out.Candidates) > limit {
		out.Candidates = out.Candidates[:limit]
	}
	for _, c := range out.Candidates {
		out.TotalImpact = round2(out.TotalImpact + c.Impact)
	}

	var b strings.Builder
	if len(out.Candidates) == 0 {
		fmt.Fprintf(&b, "No unreviewed splits move more than %.2f NOK.", minImpact)
	} else {
		fmt.Fprintf(&b, "%d splits worth reviewing, %.2f NOK at stake in total.\n"+
			"Impact is how much the balance moves if the split flips.\n\n",
			len(out.Candidates), out.TotalImpact)
		for _, c := range out.Candidates {
			fmt.Fprintf(&b, "id=%-5d %s %-30s %9.2f  %-9s %-20s impact %8.2f",
				c.TransactionID, c.Date, truncate(c.Description, 30), c.Amount,
				sharedLabel(c.IsShared), truncate(c.Category, 20), c.Impact)
			if c.Abroad {
				fmt.Fprintf(&b, "  [%s]", c.Country)
			}
			if c.PartnerSameDay > 0 {
				fmt.Fprintf(&b, "  [partner spent %d× same day]", c.PartnerSameDay)
			}
			if c.PartnerSameTrip {
				b.WriteString("  [both abroad]")
			}
			b.WriteString("\n")
		}
		b.WriteString("\nThese signals are hints, not answers. If the evidence is thin, " +
			"ask the user rather than guessing, then record the decisions with set_splits.")
	}

	return text(b.String()), out, nil
}

// isAbroad treats anything outside Norway as travel. Rows uploaded before the
// location columns existed have an empty country and are simply not flagged.
func isAbroad(country string) bool {
	c := strings.ToUpper(strings.TrimSpace(country))
	return c != "" && c != "NORWAY" && c != "NORGE" && c != "NO"
}

// ── set_splits ─────────────────────────────────────────────────────────────────

type splitArg struct {
	TransactionID int64  `json:"transaction_id" jsonschema:"id from review_splits or search_transactions"`
	IsShared      bool   `json:"is_shared" jsonschema:"true to split between both people, false for the cardholder alone"`
	Reason        string `json:"reason" jsonschema:"short note on why, for example: solo work trip, or joint furniture purchase"`
}

type setSplitsArgs struct {
	Splits []splitArg `json:"splits" jsonschema:"all the split decisions to record; applied together or not at all"`
}

type setSplitsOut struct {
	Updated     int     `json:"updated"`
	BalanceMove float64 `json:"balance_moved"`
	Currency    string  `json:"currency"`
}

func (t *mcpTools) setSplits(ctx context.Context, req *mcp.CallToolRequest, in setSplitsArgs) (
	*mcp.CallToolResult, setSplitsOut, error,
) {
	if len(in.Splits) == 0 {
		return nil, setSplitsOut{}, fmt.Errorf("no splits given")
	}
	if len(in.Splits) > 500 {
		return nil, setSplitsOut{}, fmt.Errorf("too many splits in one call (%d, max 500)", len(in.Splits))
	}

	ratios := map[string]float64{}
	for _, p := range SplitRatio {
		ratios[p.Name] = p.Ratio
	}

	type pending struct {
		tx     *Transaction
		shared bool
		reason string
		move   float64
	}

	var todo []pending
	for _, s := range in.Splits {
		if strings.TrimSpace(s.Reason) == "" {
			return nil, setSplitsOut{}, fmt.Errorf("transaction %d: a reason is required, so the "+
				"decision can be understood later", s.TransactionID)
		}
		tx, err := getTransaction(t.db, s.TransactionID)
		if err != nil {
			return nil, setSplitsOut{}, err
		}
		if tx == nil {
			return nil, setSplitsOut{}, fmt.Errorf("no transaction with id %d", s.TransactionID)
		}

		move := 0.0
		if tx.IsShared != s.IsShared {
			move = round2(tx.Amount * (1 - ratios[tx.Cardholder]))
		}
		todo = append(todo, pending{tx: tx, shared: s.IsShared, reason: strings.TrimSpace(s.Reason), move: move})
	}

	dbTransaction, err := t.db.Begin()
	if err != nil {
		return nil, setSplitsOut{}, err
	}
	defer dbTransaction.Rollback()

	stmt, err := dbTransaction.Prepare(`
		UPDATE "transaction"
		SET is_shared    = $1,
		    split_locked = true,
		    split_reason = $2,
		    modified     = (category <> orig_category
		                    OR $1 <> orig_is_shared
		                    OR cardholder <> orig_cardholder)
		WHERE id = $3`)
	if err != nil {
		return nil, setSplitsOut{}, err
	}
	defer stmt.Close()

	out := setSplitsOut{Currency: "NOK"}
	var b strings.Builder

	for _, p := range todo {
		if _, err := stmt.Exec(p.shared, p.reason, p.tx.ID); err != nil {
			return nil, setSplitsOut{}, err
		}
		out.Updated++
		out.BalanceMove = round2(out.BalanceMove + p.move)
		fmt.Fprintf(&b, "id=%-5d %-30s → %s (%s)\n",
			p.tx.ID, truncate(p.tx.Description, 30), sharedLabel(p.shared), p.reason)
	}

	if err := dbTransaction.Commit(); err != nil {
		return nil, setSplitsOut{}, err
	}

	return text(fmt.Sprintf("Recorded %d split decisions, moving %.2f NOK between the two "+
		"balances. These are now locked and will not be changed by rule runs.\n\n%s",
		out.Updated, out.BalanceMove, b.String())), out, nil
}

// ── create_categories ──────────────────────────────────────────────────────────

type newCategoryArg struct {
	Name          string `json:"name" jsonschema:"the new category name, in the same style as the existing ones"`
	SharedDefault bool   `json:"shared_default" jsonschema:"whether transactions in this category should be split by default"`
	Note          string `json:"note,omitempty" jsonschema:"short note on what belongs in this category"`
}

type createCategoriesArgs struct {
	Categories []newCategoryArg `json:"categories" jsonschema:"the categories to create; send them all in one call"`
}

type createCategoriesOut struct {
	Created   []Category         `json:"created"`
	Conflicts []CategoryConflict `json:"conflicts"`
}

func (t *mcpTools) createCategories(ctx context.Context, req *mcp.CallToolRequest, in createCategoriesArgs) (
	*mcp.CallToolResult, createCategoriesOut, error,
) {
	if len(in.Categories) == 0 {
		return nil, createCategoriesOut{}, fmt.Errorf("no categories given")
	}
	if len(in.Categories) > 25 {
		return nil, createCategoriesOut{}, fmt.Errorf("too many categories in one call (%d, max 25)", len(in.Categories))
	}

	cats := make([]Category, 0, len(in.Categories))
	for _, c := range in.Categories {
		cats = append(cats, Category{Name: c.Name, SharedDefault: c.SharedDefault, Note: c.Note})
	}

	created, conflicts, err := createCategories(t.db, cats)
	if err != nil {
		return nil, createCategoriesOut{}, err
	}

	out := createCategoriesOut{Created: created, Conflicts: conflicts}
	if out.Created == nil {
		out.Created = []Category{}
	}
	if out.Conflicts == nil {
		out.Conflicts = []CategoryConflict{}
	}

	var b strings.Builder
	if len(created) > 0 {
		fmt.Fprintf(&b, "Created %d categories:\n", len(created))
		for _, c := range created {
			fmt.Fprintf(&b, "  %-26s %s by default\n", c.Name, sharedLabel(c.SharedDefault))
		}
	}
	for _, c := range conflicts {
		fmt.Fprintf(&b, "Skipped %q: %q already exists and means the same thing. Use it instead.\n",
			c.Requested, c.Existing)
	}
	if len(created) > 0 {
		b.WriteString("\nNothing is categorized yet — follow up with apply_category_rules.")
	}

	return text(b.String()), out, nil
}
