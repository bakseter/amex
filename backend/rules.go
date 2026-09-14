package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// Rules stored in the database take precedence over the ones compiled into
// parse.go, so a correction the agent makes once also applies to next month's
// upload. They are cached in memory and reloaded whenever they change.

type CategoryRule struct {
	ID        int64
	Pattern   string
	Category  string
	IsShared  *bool // nil = inherit the category's default
	CreatedAt time.Time

	re *regexp.Regexp
}

var dynamicRules atomic.Pointer[[]CategoryRule]

// compileRule validates a user-supplied pattern before it goes anywhere near
// the database. Go's regexp is RE2, so there is no catastrophic backtracking to
// worry about; the real risk is a pattern broad enough to swallow everything.
func compileRule(pattern, category string, isShared *bool) (CategoryRule, error) {
	pattern = strings.TrimSpace(pattern)

	switch {
	case pattern == "":
		return CategoryRule{}, fmt.Errorf("pattern is empty")
	case len(pattern) > 200:
		return CategoryRule{}, fmt.Errorf("pattern is too long (max 200 characters)")
	case !validCategory(category):
		return CategoryRule{}, fmt.Errorf("unknown category %q; call list_categories for valid names", category)
	}

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return CategoryRule{}, fmt.Errorf("pattern %q is not a valid regular expression: %w", pattern, err)
	}
	if re.MatchString("") {
		return CategoryRule{}, fmt.Errorf("pattern %q matches every description; make it more specific", pattern)
	}

	return CategoryRule{Pattern: pattern, Category: category, IsShared: isShared, re: re}, nil
}

func loadCategoryRules(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, pattern, category, is_shared, created_at FROM category_rule ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var out []CategoryRule
	for rows.Next() {
		var r CategoryRule
		var shared sql.NullBool
		if err := rows.Scan(&r.ID, &r.Pattern, &r.Category, &shared, &r.CreatedAt); err != nil {
			return err
		}
		if shared.Valid {
			v := shared.Bool
			r.IsShared = &v
		}
		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			// A rule that no longer compiles should not take the server down.
			slog.Warn("skipping invalid category rule", "id", r.ID, "pattern", r.Pattern, "err", err)
			continue
		}
		r.re = re
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	dynamicRules.Store(&out)
	return nil
}

func currentRules() []CategoryRule {
	if p := dynamicRules.Load(); p != nil {
		return *p
	}
	return nil
}

// saveCategoryRules upserts by pattern and returns the stored rules.
func saveCategoryRules(db *sql.DB, rules []CategoryRule) ([]CategoryRule, error) {
	dbTx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer dbTx.Rollback()

	saved := make([]CategoryRule, 0, len(rules))
	for _, r := range rules {
		var id int64
		var created time.Time
		var shared any
		if r.IsShared != nil {
			shared = *r.IsShared
		}
		err := dbTx.QueryRow(`
			INSERT INTO category_rule (pattern, category, is_shared) VALUES ($1, $2, $3)
			ON CONFLICT (pattern) DO UPDATE
			SET category = EXCLUDED.category, is_shared = EXCLUDED.is_shared
			RETURNING id, created_at`, r.Pattern, r.Category, shared).Scan(&id, &created)
		if err != nil {
			return nil, err
		}
		r.ID, r.CreatedAt = id, created
		saved = append(saved, r)
	}

	if err := dbTx.Commit(); err != nil {
		return nil, err
	}
	return saved, loadCategoryRules(db)
}

func deleteCategoryRules(db *sql.DB, ids []int64) (int64, error) {
	res, err := db.Exec(`DELETE FROM category_rule WHERE id = ANY($1)`, int64Array(ids))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, loadCategoryRules(db)
}

// int64Array renders ids as a Postgres array literal, avoiding a dependency on
// a driver-specific array type.
func int64Array(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ── Applying rules to stored transactions ──────────────────────────────────────

type RuleChange struct {
	TransactionID int64   `json:"transaction_id"`
	Description   string  `json:"description"`
	Amount        float64 `json:"amount"`
	From          string  `json:"from_category"`
	To            string  `json:"to_category"`
	FromShared    bool    `json:"from_shared"`
	ToShared      bool    `json:"to_shared"`
	SplitLocked   bool    `json:"split_locked"`
	Pattern       string  `json:"matched_pattern"`
}

// evaluateRules works out what a set of rules would change, without writing.
// The per-rule match counts come back alongside, so a pattern that matches
// nothing (usually a typo) is visible rather than silently doing nothing.
func evaluateRules(db *sql.DB, rules []CategoryRule, scopeAll bool) ([]RuleChange, map[string]int, error) {
	q := `SELECT id, description, amount, category, is_shared, split_locked FROM "transaction"`
	if !scopeAll {
		q += ` WHERE category = 'Uncategorized'`
	}
	q += ` ORDER BY amount DESC`

	rows, err := db.Query(q)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	changes := []RuleChange{}
	counts := map[string]int{}
	for _, r := range rules {
		counts[r.Pattern] = 0
	}

	for rows.Next() {
		var id int64
		var desc, category string
		var amount float64
		var isShared, splitLocked bool
		if err := rows.Scan(&id, &desc, &amount, &category, &isShared, &splitLocked); err != nil {
			return nil, nil, err
		}

		lower := strings.ToLower(desc)
		for _, r := range rules {
			if !r.re.MatchString(lower) {
				continue
			}
			counts[r.Pattern]++ // count every match, even a no-op

			// A deliberately decided split survives a rule run; only the
			// category moves.
			newShared := isShared
			if !splitLocked {
				if r.IsShared != nil {
					newShared = *r.IsShared
				} else {
					newShared = sharedDefault(r.Category)
				}
			}

			if r.Category != category || newShared != isShared {
				changes = append(changes, RuleChange{
					TransactionID: id, Description: desc, Amount: amount,
					From: category, To: r.Category,
					FromShared: isShared, ToShared: newShared,
					SplitLocked: splitLocked,
					Pattern:     r.Pattern,
				})
			}
			break // first matching rule wins, same as classify
		}
	}

	return changes, counts, rows.Err()
}

// applyChanges writes a set of changes in one database transaction, so a
// failure part-way through leaves nothing half-applied.
func applyChanges(db *sql.DB, changes []RuleChange) error {
	dbTx, err := db.Begin()
	if err != nil {
		return err
	}
	defer dbTx.Rollback()

	stmt, err := dbTx.Prepare(`
		UPDATE "transaction"
		SET category  = $1,
		    is_shared = $2,
		    modified  = ($1 <> orig_category
		                 OR $2 <> orig_is_shared
		                 OR cardholder <> orig_cardholder)
		WHERE id = $3`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range changes {
		if _, err := stmt.Exec(c.To, c.ToShared, c.TransactionID); err != nil {
			return err
		}
	}

	return dbTx.Commit()
}

// ── Grouping uncategorized spend by merchant ───────────────────────────────────

var (
	nonAlnum = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	hasDigit = regexp.MustCompile(`\d`)
)

// merchantKey collapses "Kiwi 381 Sorgenfrigata Oslo" and "Kiwi 220 Bogstadv
// Oslo" onto the same key, so the agent sees one decision instead of two.
func merchantKey(description string) string {
	s := strings.ToLower(description)
	s = nonAlnum.ReplaceAllString(s, " ")

	var words []string
	for _, w := range strings.Fields(s) {
		if hasDigit.MatchString(w) {
			continue // store numbers, postcodes, card fragments
		}
		words = append(words, w)
		if len(words) == 3 {
			break
		}
	}
	if len(words) == 0 {
		return strings.TrimSpace(s)
	}
	return strings.Join(words, " ")
}

// suggestedPattern turns a merchant key into a conservative regex literal.
func suggestedPattern(key string) string {
	words := strings.Fields(key)
	if len(words) > 2 {
		words = words[:2]
	}
	return regexp.QuoteMeta(strings.Join(words, " "))
}
