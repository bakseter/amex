package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tools exist so a categorization pass is one approval instead of forty.
// The shape is: survey (list_uncategorized) → dry run (preview_category_rules)
// → one write (apply_category_rules).

// ── list_uncategorized ─────────────────────────────────────────────────────────

type uncategorizedArgs struct {
	InvoiceID int64 `json:"invoice_id,omitempty" jsonschema:"limit to one statement; omit or 0 for all statements"`
	Limit     int   `json:"limit,omitempty" jsonschema:"maximum merchant groups to return, default 40"`
}

type merchantGroup struct {
	Merchant         string   `json:"merchant"`
	SuggestedPattern string   `json:"suggested_pattern"`
	Count            int      `json:"count"`
	Total            float64  `json:"total"`
	Examples         []string `json:"examples"`
	TransactionIDs   []int64  `json:"transaction_ids"`
}

type uncategorizedOut struct {
	Merchants  []merchantGroup `json:"merchants"`
	TotalRows  int             `json:"total_rows"`
	TotalValue float64         `json:"total_value"`
	Truncated  bool            `json:"truncated"`
	Currency   string          `json:"currency"`
}

func (t *mcpTools) listUncategorized(ctx context.Context, req *mcp.CallToolRequest, in uncategorizedArgs) (
	*mcp.CallToolResult, uncategorizedOut, error,
) {
	limit := in.Limit
	if limit <= 0 {
		limit = 40
	}

	q := `SELECT id, description, amount FROM "transaction" WHERE category = 'Uncategorized'`
	var args []any
	if in.InvoiceID > 0 {
		q += ` AND invoice_id = $1`
		args = append(args, in.InvoiceID)
	}

	rows, err := t.db.Query(q, args...)
	if err != nil {
		return nil, uncategorizedOut{}, err
	}
	defer rows.Close()

	groups := map[string]*merchantGroup{}
	out := uncategorizedOut{Currency: "NOK", Merchants: []merchantGroup{}}

	for rows.Next() {
		var id int64
		var desc string
		var amount float64
		if err := rows.Scan(&id, &desc, &amount); err != nil {
			return nil, uncategorizedOut{}, err
		}

		key := merchantKey(desc)
		g, ok := groups[key]
		if !ok {
			g = &merchantGroup{Merchant: key, SuggestedPattern: suggestedPattern(key)}
			groups[key] = g
		}
		g.Count++
		g.Total = round2(g.Total + amount)
		g.TransactionIDs = append(g.TransactionIDs, id)
		if len(g.Examples) < 3 && !contains(g.Examples, desc) {
			g.Examples = append(g.Examples, desc)
		}

		out.TotalRows++
		out.TotalValue = round2(out.TotalValue + amount)
	}
	if err := rows.Err(); err != nil {
		return nil, uncategorizedOut{}, err
	}

	for _, g := range groups {
		out.Merchants = append(out.Merchants, *g)
	}
	sort.Slice(out.Merchants, func(i, j int) bool {
		return out.Merchants[i].Total > out.Merchants[j].Total
	})
	if len(out.Merchants) > limit {
		out.Merchants = out.Merchants[:limit]
		out.Truncated = true
	}

	var b strings.Builder
	if out.TotalRows == 0 {
		b.WriteString("Nothing is uncategorized.")
	} else {
		fmt.Fprintf(&b, "%d uncategorized transactions worth %.2f NOK, in %d merchant groups:\n\n",
			out.TotalRows, out.TotalValue, len(groups))
		for _, g := range out.Merchants {
			fmt.Fprintf(&b, "%-28s %3d × %9.2f  suggested pattern: %s\n    e.g. %s\n",
				truncate(g.Merchant, 28), g.Count, g.Total, g.SuggestedPattern,
				strings.Join(g.Examples, " | "))
		}
		b.WriteString("\nPropose one rule per merchant, check them with preview_category_rules, " +
			"then write them all in a single apply_category_rules call.")
	}

	return text(b.String()), out, nil
}

// ── preview_category_rules ─────────────────────────────────────────────────────

type ruleArg struct {
	Pattern  string `json:"pattern" jsonschema:"case-insensitive regular expression matched against the merchant description"`
	Category string `json:"category" jsonschema:"category to assign, exactly as spelled by list_categories"`
	IsShared *bool  `json:"is_shared,omitempty" jsonschema:"override the split for this merchant; omit to inherit the category default"`
}

type rulesArgs struct {
	Rules []ruleArg `json:"rules" jsonschema:"the rules to evaluate, applied in order with the first match winning"`
	Scope string    `json:"scope,omitempty" jsonschema:"uncategorized (default) to only fill in gaps, or all to also recategorize transactions that already have a category, including ones a human edited by hand"`
}

type previewOut struct {
	Changes     []RuleChange   `json:"changes"`
	MatchCounts map[string]int `json:"match_counts"`
	DeadRules   []string       `json:"dead_rules"`
	TotalValue  float64        `json:"total_value"`
	Currency    string         `json:"currency"`
}

func (t *mcpTools) previewCategoryRules(ctx context.Context, req *mcp.CallToolRequest, in rulesArgs) (
	*mcp.CallToolResult, previewOut, error,
) {
	rules, scopeAll, err := parseRuleArgs(in)
	if err != nil {
		return nil, previewOut{}, err
	}

	changes, counts, err := evaluateRules(t.db, rules, scopeAll)
	if err != nil {
		return nil, previewOut{}, err
	}

	out := previewOut{Changes: changes, MatchCounts: counts, Currency: "NOK", DeadRules: []string{}}
	for _, c := range changes {
		out.TotalValue = round2(out.TotalValue + c.Amount)
	}
	for _, r := range rules {
		if counts[r.Pattern] == 0 {
			out.DeadRules = append(out.DeadRules, r.Pattern)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d transactions would change, worth %.2f NOK.\n\n", len(changes), out.TotalValue)
	for i, c := range changes {
		if i == 30 {
			fmt.Fprintf(&b, "... and %d more\n", len(changes)-30)
			break
		}
		split := ""
		if c.ToShared != c.FromShared {
			split = fmt.Sprintf("  [%s → %s]", sharedLabel(c.FromShared), sharedLabel(c.ToShared))
		} else if c.SplitLocked {
			split = "  [split locked, left alone]"
		}
		fmt.Fprintf(&b, "id=%-5d %-34s %9.2f  %s → %s%s\n",
			c.TransactionID, truncate(c.Description, 34), c.Amount, c.From, c.To, split)
	}
	if len(out.DeadRules) > 0 {
		fmt.Fprintf(&b, "\nThese patterns matched nothing and are probably wrong: %s\n",
			strings.Join(out.DeadRules, ", "))
	}
	if len(changes) > 0 {
		b.WriteString("\nIf this looks right, pass the same rules to apply_category_rules.")
	}

	return text(b.String()), out, nil
}

// ── apply_category_rules ───────────────────────────────────────────────────────

type applyArgs struct {
	Rules  []ruleArg `json:"rules" jsonschema:"the rules to save and apply, in priority order"`
	Scope  string    `json:"scope,omitempty" jsonschema:"uncategorized (default) or all"`
	Persist *bool    `json:"persist,omitempty" jsonschema:"save the rules so future uploads use them too; defaults to true"`
}

type applyOut struct {
	Updated    int            `json:"transactions_updated"`
	RulesSaved int            `json:"rules_saved"`
	TotalValue float64        `json:"total_value"`
	ByCategory map[string]int `json:"by_category"`
	DeadRules  []string       `json:"dead_rules"`
}

func (t *mcpTools) applyCategoryRules(ctx context.Context, req *mcp.CallToolRequest, in applyArgs) (
	*mcp.CallToolResult, applyOut, error,
) {
	rules, scopeAll, err := parseRuleArgs(rulesArgs{Rules: in.Rules, Scope: in.Scope})
	if err != nil {
		return nil, applyOut{}, err
	}

	changes, counts, err := evaluateRules(t.db, rules, scopeAll)
	if err != nil {
		return nil, applyOut{}, err
	}

	if err := applyChanges(t.db, changes); err != nil {
		return nil, applyOut{}, err
	}

	out := applyOut{
		Updated:    len(changes),
		ByCategory: map[string]int{},
		DeadRules:  []string{},
	}
	for _, c := range changes {
		out.ByCategory[c.To]++
		out.TotalValue = round2(out.TotalValue + c.Amount)
	}
	for _, r := range rules {
		if counts[r.Pattern] == 0 {
			out.DeadRules = append(out.DeadRules, r.Pattern)
		}
	}

	persist := in.Persist == nil || *in.Persist
	if persist {
		saved, err := saveCategoryRules(t.db, rules)
		if err != nil {
			// The transactions are already updated; say so rather than
			// implying nothing happened.
			return nil, out, fmt.Errorf("updated %d transactions, but saving the rules failed: %w",
				len(changes), err)
		}
		out.RulesSaved = len(saved)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Updated %d transactions (%.2f NOK).\n", out.Updated, out.TotalValue)
	for cat, n := range out.ByCategory {
		fmt.Fprintf(&b, "  %-26s %d\n", cat, n)
	}
	if persist {
		fmt.Fprintf(&b, "\nSaved %d rules; future uploads will categorize these merchants automatically.\n",
			out.RulesSaved)
	}
	if len(out.DeadRules) > 0 {
		fmt.Fprintf(&b, "Patterns that matched nothing: %s\n", strings.Join(out.DeadRules, ", "))
	}

	return text(b.String()), out, nil
}

// ── delete_category_rules ──────────────────────────────────────────────────────

type deleteRulesArgs struct {
	RuleIDs []int64 `json:"rule_ids" jsonschema:"ids of the saved rules to remove, from list_categories"`
}

type deleteRulesOut struct {
	Deleted int64 `json:"deleted"`
}

func (t *mcpTools) deleteCategoryRules(ctx context.Context, req *mcp.CallToolRequest, in deleteRulesArgs) (
	*mcp.CallToolResult, deleteRulesOut, error,
) {
	if len(in.RuleIDs) == 0 {
		return nil, deleteRulesOut{}, fmt.Errorf("no rule_ids given")
	}

	n, err := deleteCategoryRules(t.db, in.RuleIDs)
	if err != nil {
		return nil, deleteRulesOut{}, err
	}

	return text(fmt.Sprintf("Deleted %d rules. Transactions already categorized by them keep "+
		"their category; run apply_category_rules with scope=all to redo them.", n)),
		deleteRulesOut{Deleted: n}, nil
}

// ── update_transactions ────────────────────────────────────────────────────────

type txUpdateArg struct {
	TransactionID int64  `json:"transaction_id" jsonschema:"id from search_transactions"`
	Category      string `json:"category,omitempty" jsonschema:"new category, exactly as spelled by list_categories"`
	Cardholder    string `json:"cardholder,omitempty" jsonschema:"new cardholder"`
	IsShared      *bool  `json:"is_shared,omitempty" jsonschema:"whether to split this transaction; defaults to what the new category implies"`
}

type batchUpdateArgs struct {
	Updates []txUpdateArg `json:"updates" jsonschema:"all the changes to make; they are applied together or not at all"`
}

type batchUpdateOut struct {
	Updated      int      `json:"updated"`
	Transactions []txInfo `json:"transactions"`
}

func (t *mcpTools) updateTransactions(ctx context.Context, req *mcp.CallToolRequest, in batchUpdateArgs) (
	*mcp.CallToolResult, batchUpdateOut, error,
) {
	if len(in.Updates) == 0 {
		return nil, batchUpdateOut{}, fmt.Errorf("no updates given")
	}
	if len(in.Updates) > 500 {
		return nil, batchUpdateOut{}, fmt.Errorf("too many updates in one call (%d, max 500)", len(in.Updates))
	}

	// Validate everything before writing anything, so a typo in the last entry
	// does not leave the first forty applied.
	txs := make([]*Tx, 0, len(in.Updates))
	for _, u := range in.Updates {
		if u.Category == "" && u.Cardholder == "" && u.IsShared == nil {
			return nil, batchUpdateOut{}, fmt.Errorf("transaction %d: nothing to change", u.TransactionID)
		}
		if u.Category != "" && !validCategory(u.Category) {
			return nil, batchUpdateOut{}, fmt.Errorf("transaction %d: unknown category %q; call list_categories",
				u.TransactionID, u.Category)
		}

		tx, err := getTransaction(t.db, u.TransactionID)
		if err != nil {
			return nil, batchUpdateOut{}, err
		}
		if tx == nil {
			return nil, batchUpdateOut{}, fmt.Errorf("no transaction with id %d", u.TransactionID)
		}

		if u.IsShared != nil {
			tx.IsShared = *u.IsShared
		}
		if u.Category != "" {
			tx.Category = u.Category
			if u.IsShared == nil {
				tx.IsShared = sharedDefault(tx.Category)
			}
		}
		if u.Cardholder != "" {
			tx.Cardholder = u.Cardholder
		}
		tx.Modified = tx.Category != tx.OrigCategory ||
			tx.IsShared != tx.OrigIsShared ||
			tx.Cardholder != tx.OrigCardholder

		txs = append(txs, tx)
	}

	dbTx, err := t.db.Begin()
	if err != nil {
		return nil, batchUpdateOut{}, err
	}
	defer dbTx.Rollback()

	stmt, err := dbTx.Prepare(`
		UPDATE "transaction" SET category = $1, is_shared = $2, cardholder = $3, modified = $4
		WHERE id = $5`)
	if err != nil {
		return nil, batchUpdateOut{}, err
	}
	defer stmt.Close()

	out := batchUpdateOut{Transactions: []txInfo{}}
	var b strings.Builder

	for _, tx := range txs {
		if _, err := stmt.Exec(tx.Category, tx.IsShared, tx.Cardholder, tx.Modified, tx.ID); err != nil {
			return nil, batchUpdateOut{}, err
		}
		out.Transactions = append(out.Transactions, txInfo{
			ID: tx.ID, InvoiceID: tx.InvoiceID, Date: tx.Date,
			Description: tx.Description, Amount: tx.Amount,
			Cardholder: tx.Cardholder, Category: tx.Category,
			IsShared: tx.IsShared,
			Owes:     owes(tx.Amount, tx.Cardholder, tx.IsShared),
		})
		fmt.Fprintf(&b, "id=%-5d %-34s → %s (%s)\n",
			tx.ID, truncate(tx.Description, 34), tx.Category, sharedLabel(tx.IsShared))
	}

	if err := dbTx.Commit(); err != nil {
		return nil, batchUpdateOut{}, err
	}
	out.Updated = len(out.Transactions)

	return text(fmt.Sprintf("Updated %d transactions.\n\n%s", out.Updated, b.String())), out, nil
}

// ── shared helpers ─────────────────────────────────────────────────────────────

func parseRuleArgs(in rulesArgs) ([]CategoryRule, bool, error) {
	if len(in.Rules) == 0 {
		return nil, false, fmt.Errorf("no rules given")
	}
	if len(in.Rules) > 100 {
		return nil, false, fmt.Errorf("too many rules in one call (%d, max 100)", len(in.Rules))
	}

	scopeAll := false
	switch strings.ToLower(strings.TrimSpace(in.Scope)) {
	case "", "uncategorized":
	case "all":
		scopeAll = true
	default:
		return nil, false, fmt.Errorf("scope must be %q or %q, got %q", "uncategorized", "all", in.Scope)
	}

	rules := make([]CategoryRule, 0, len(in.Rules))
	for _, r := range in.Rules {
		compiled, err := compileRule(r.Pattern, r.Category, r.IsShared)
		if err != nil {
			return nil, false, err
		}
		rules = append(rules, compiled)
	}
	return rules, scopeAll, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
