package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// Categories come from two places: the set implied by the rules compiled into
// parse.go, and the `category` table, which anything with write access can add
// to. The split default lives with the category rather than in a hardcoded set,
// so a new category can declare how it should be split.

type Category struct {
	Name          string    `json:"name"`
	SharedDefault bool      `json:"shared_default"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"-"`
}

var customCategories atomic.Pointer[[]Category]

func loadCategories(database *sql.DB) error {
	rows, err := database.Query(`SELECT name, shared_default, note, created_at FROM category ORDER BY name`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Name, &c.SharedDefault, &c.Note, &c.CreatedAt); err != nil {
			return err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	customCategories.Store(&out)
	return nil
}

func currentCustomCategories() []Category {
	if p := customCategories.Load(); p != nil {
		return *p
	}
	return nil
}

// AllCategories is every category name, built-in and custom, sorted.
func AllCategories() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range builtinCategories {
		seen[c] = true
		out = append(out, c)
	}
	for _, c := range currentCustomCategories() {
		if !seen[c.Name] {
			seen[c.Name] = true
			out = append(out, c.Name)
		}
	}
	sort.Strings(out)
	return out
}

func validCategory(name string) bool {
	for _, c := range AllCategories() {
		if c == name {
			return true
		}
	}
	return false
}

// sharedDefault says whether a category is split by default. Custom categories
// carry their own answer; built-in ones use the set in parse.go.
func sharedDefault(category string) bool {
	for _, c := range currentCustomCategories() {
		if c.Name == category {
			return c.SharedDefault
		}
	}
	return SharedCategories[category]
}

// SharedCategoryList returns every category that is split by default, sorted.
func SharedCategoryList() []string {
	var out []string
	for _, c := range AllCategories() {
		if sharedDefault(c) {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func splitRatioMap() map[string]float64 {
	out := map[string]float64{}
	for _, p := range SplitRatio {
		out[p.Name] = p.Ratio
	}
	return out
}

// ── Creating categories ────────────────────────────────────────────────────────

// A new category is cheap to create and expensive to un-create: once rules and
// transactions point at "Coffee", having "Café" as well means every summary is
// split across two lines that should be one. So creation checks for near
// duplicates and refuses rather than guessing.

var (
	nonAlnumName = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	accentFold   = strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
)

// normalizeName collapses the differences that make two category names look
// distinct to a database and identical to a person: case, punctuation, accents
// and a trailing plural s. Norwegian æ, ø and å are left alone, since they are
// letters in their own right rather than accented vowels.
func normalizeName(s string) string {
	s = accentFold.Replace(strings.ToLower(strings.TrimSpace(s)))
	s = nonAlnumName.ReplaceAllString(s, "")
	return strings.TrimSuffix(s, "s")
}

type CategoryConflict struct {
	Requested string `json:"requested"`
	Existing  string `json:"existing"`
}

// createCategories adds categories that do not already exist. It returns what
// was created and what collided with an existing name.
func createCategories(db *sql.DB, cats []Category) ([]Category, []CategoryConflict, error) {
	existing := map[string]string{} // normalized -> actual name
	for _, c := range AllCategories() {
		existing[normalizeName(c)] = c
	}

	var toCreate []Category
	var conflicts []CategoryConflict
	seen := map[string]bool{}

	for _, c := range cats {
		c.Name = strings.TrimSpace(c.Name)

		switch {
		case c.Name == "":
			return nil, nil, fmt.Errorf("category name is empty")
		case len(c.Name) > 60:
			return nil, nil, fmt.Errorf("category name %q is too long (max 60 characters)", c.Name)
		case strings.EqualFold(c.Name, "Uncategorized"):
			return nil, nil, fmt.Errorf(`"Uncategorized" is reserved`)
		}

		key := normalizeName(c.Name)
		if key == "" {
			return nil, nil, fmt.Errorf("category name %q has no letters or digits", c.Name)
		}
		if match, ok := existing[key]; ok {
			conflicts = append(conflicts, CategoryConflict{Requested: c.Name, Existing: match})
			continue
		}
		if seen[key] {
			continue // duplicate within this request
		}

		seen[key] = true
		toCreate = append(toCreate, c)
	}

	if len(toCreate) == 0 {
		return nil, conflicts, nil
	}

	dbTransaction, err := db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer dbTransaction.Rollback()

	for _, c := range toCreate {
		_, err := dbTransaction.Exec(
			`INSERT INTO category (name, shared_default, note) VALUES ($1, $2, $3)
			 ON CONFLICT (name) DO NOTHING`,
			c.Name, c.SharedDefault, c.Note,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := dbTransaction.Commit(); err != nil {
		return nil, nil, err
	}
	return toCreate, conflicts, loadCategories(db)
}
