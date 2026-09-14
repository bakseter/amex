package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The MCP server runs in this same process and talks to Postgres directly,
// rather than looping back through the REST API over HTTP.

type mcpTools struct{ db *sql.DB }

// mountMCP registers the streamable HTTP MCP endpoint on the router.
func mountMCP(r *gin.Engine, db *sql.DB, path string) {
	server := newMCPServer(db)
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil,
	)
	r.Any(path, gin.WrapH(handler))
}

func newMCPServer(db *sql.DB) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "amex",
		Version: "0.1.0",
	}, nil)

	t := &mcpTools{db: db}

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_invoices",
		Description: "List the uploaded Amex statements, newest first. " +
			"Call this first to find the invoice_id for a given month.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.listInvoices)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_invoice_summary",
		Description: "Totals for one statement broken down by category, plus how much " +
			"each person owes. Use this for questions like 'what did we spend on groceries' " +
			"or 'how much does this person owe'.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.getInvoiceSummary)

	mcp.AddTool(s, &mcp.Tool{
		Name: "search_transactions",
		Description: "Search individual transactions. All filters are optional and combine " +
			"with AND. Omit invoice_id to search across every statement.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.searchTransactions)

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_categories",
		Description: "The valid category names, which are treated as shared by default, " +
			"and the known cardholders. Call this before filtering or updating by category.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.listCategories)

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_uncategorized",
		Description: "Uncategorized spending grouped by merchant, largest first, with a " +
			"suggested regex for each. Start here when asked to clean up categories: it turns " +
			"hundreds of rows into a handful of decisions.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.listUncategorized)

	mcp.AddTool(s, &mcp.Tool{
		Name: "preview_category_rules",
		Description: "Dry run: show exactly which transactions a set of rules would " +
			"recategorize, without writing anything. Always run this before apply_category_rules.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.previewCategoryRules)

	mcp.AddTool(s, &mcp.Tool{
		Name: "apply_category_rules",
		Description: "Save a set of category rules and apply them to existing transactions, " +
			"in one operation. Saved rules also categorize future uploads, so prefer this over " +
			"editing transactions one at a time. Send every rule in a single call.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.applyCategoryRules)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_category_rules",
		Description: "Remove saved category rules by id. Use when a rule turned out to be too broad.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.deleteCategoryRules)

	mcp.AddTool(s, &mcp.Tool{
		Name: "review_splits",
		Description: "Transactions whose shared/personal flag is worth a second look, " +
			"ranked by how much money moves if the flag flips, with context on whether " +
			"the other person was spending that day and whether the charge was abroad. " +
			"Use this instead of reviewing every row.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.reviewSplits)

	mcp.AddTool(s, &mcp.Tool{
		Name: "set_splits",
		Description: "Record split decisions for specific transactions, with a reason for each. " +
			"These are locked afterwards and survive later rule runs. Send every decision in one call.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.setSplits)

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_categories",
		Description: "Create new categories when nothing existing fits. Near-duplicates of " +
			"existing names are rejected, because two names for one thing splits every summary " +
			"in half. Check list_categories first and prefer an existing name.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.createCategories)

	mcp.AddTool(s, &mcp.Tool{
		Name: "update_transactions",
		Description: "Change the category, shared flag or cardholder of specific transactions. " +
			"Takes a list and applies it as one all-or-nothing write, so send every change in a " +
			"single call rather than one call per transaction. For anything merchant-shaped, " +
			"apply_category_rules is better because it also covers future statements.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.updateTransactions)

	return s
}

// ── list_invoices ──────────────────────────────────────────────────────────────

type noArgs struct{}

type invoiceInfo struct {
	InvoiceID        int64  `json:"invoice_id"`
	Filename         string `json:"filename"`
	UploadedAt       string `json:"uploaded_at"`
	TransactionCount int    `json:"transaction_count"`
}

type listInvoicesOut struct {
	Invoices []invoiceInfo `json:"invoices"`
}

func (t *mcpTools) listInvoices(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (
	*mcp.CallToolResult, listInvoicesOut, error,
) {
	rows, err := listInvoiceRows(t.db)
	if err != nil {
		return nil, listInvoicesOut{}, err
	}

	out := listInvoicesOut{Invoices: []invoiceInfo{}}
	var b strings.Builder
	for _, r := range rows {
		out.Invoices = append(out.Invoices, invoiceInfo{
			InvoiceID:        r.ID,
			Filename:         r.Filename,
			UploadedAt:       r.UploadedAt.Format(isoTime),
			TransactionCount: r.TransactionCount,
		})
		fmt.Fprintf(&b, "invoice_id=%d  %s  (%d transactions, uploaded %s)\n",
			r.ID, r.Filename, r.TransactionCount, r.UploadedAt.Format("2006-01-02"))
	}
	if len(rows) == 0 {
		b.WriteString("No statements have been uploaded yet.")
	}

	return text(b.String()), out, nil
}

// ── get_invoice_summary ────────────────────────────────────────────────────────

type invoiceIDArgs struct {
	InvoiceID int64 `json:"invoice_id" jsonschema:"id of the statement, from list_invoices"`
}

type categoryTotal struct {
	Category string             `json:"category"`
	Total    float64            `json:"total"`
	Owes     map[string]float64 `json:"owes"`
}

type summaryOut struct {
	InvoiceID    int64              `json:"invoice_id"`
	Filename     string             `json:"filename"`
	GrandTotal   float64            `json:"grand_total"`
	ByCategory   []categoryTotal    `json:"by_category"`
	PersonTotals map[string]float64 `json:"person_totals"`
	Currency     string             `json:"currency"`
}

func (t *mcpTools) getInvoiceSummary(ctx context.Context, req *mcp.CallToolRequest, in invoiceIDArgs) (
	*mcp.CallToolResult, summaryOut, error,
) {
	exists, filename, err := invoiceExists(t.db, in.InvoiceID)
	if err != nil {
		return nil, summaryOut{}, err
	}
	if !exists {
		return nil, summaryOut{}, fmt.Errorf("no invoice with id %d; call list_invoices to see valid ids", in.InvoiceID)
	}

	txs, err := listTransactions(t.db, in.InvoiceID)
	if err != nil {
		return nil, summaryOut{}, err
	}

	order, totals := categoryTotals(txs)
	persons := PersonNames()

	out := summaryOut{
		InvoiceID:    in.InvoiceID,
		Filename:     filename,
		PersonTotals: personTotals(txs),
		Currency:     "NOK",
		ByCategory:   []categoryTotal{},
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s (invoice_id=%d), %d transactions\n\n", filename, in.InvoiceID, len(txs))

	for _, cat := range order {
		ct := categoryTotal{Category: cat, Total: totals[cat]["total"], Owes: map[string]float64{}}
		for _, p := range persons {
			ct.Owes[p] = round2(totals[cat][p])
		}
		out.ByCategory = append(out.ByCategory, ct)
		out.GrandTotal += ct.Total

		fmt.Fprintf(&b, "%-26s %10.2f\n", cat, ct.Total)
	}

	fmt.Fprintf(&b, "%-26s %10.2f\n\n", "TOTAL", out.GrandTotal)
	for _, p := range persons {
		fmt.Fprintf(&b, "%s owes %.2f NOK\n", p, out.PersonTotals[p])
	}

	return text(b.String()), out, nil
}

// ── search_transactions ────────────────────────────────────────────────────────

type searchArgs struct {
	InvoiceID           int64   `json:"invoice_id,omitempty" jsonschema:"limit to one statement; omit or 0 to search all statements"`
	Category            string  `json:"category,omitempty" jsonschema:"exact category name, for example Groceries"`
	Cardholder          string  `json:"cardholder,omitempty" jsonschema:"whose card was used, for example Bob"`
	DescriptionContains string  `json:"description_contains,omitempty" jsonschema:"case-insensitive substring of the merchant name"`
	MinAmount           float64 `json:"min_amount,omitempty" jsonschema:"only transactions at or above this amount in NOK"`
	MaxAmount           float64 `json:"max_amount,omitempty" jsonschema:"only transactions at or below this amount in NOK"`
	SharedOnly          bool    `json:"shared_only,omitempty" jsonschema:"only transactions that are split between both people"`
	Limit               int     `json:"limit,omitempty" jsonschema:"maximum rows to return, default 50, maximum 200"`
}

type txInfo struct {
	ID          int64              `json:"id"`
	InvoiceID   int64              `json:"invoice_id"`
	Date        string             `json:"date"`
	Description string             `json:"description"`
	Amount      float64            `json:"amount"`
	Cardholder  string             `json:"cardholder"`
	Category    string             `json:"category"`
	IsShared    bool               `json:"is_shared"`
	Owes        map[string]float64 `json:"owes"`
}

type searchOut struct {
	Transactions []txInfo `json:"transactions"`
	Count        int      `json:"count"`
	Total        float64  `json:"total"`
	Truncated    bool     `json:"truncated"`
	Currency     string   `json:"currency"`
}

func (t *mcpTools) searchTransactions(ctx context.Context, req *mcp.CallToolRequest, in searchArgs) (
	*mcp.CallToolResult, searchOut, error,
) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	q := `SELECT ` + txColumns + ` FROM "transaction" WHERE 1 = 1`
	var args []any

	where := func(clause string, v any) {
		args = append(args, v)
		q += fmt.Sprintf(clause, len(args))
	}

	if in.InvoiceID > 0 {
		where(` AND invoice_id = $%d`, in.InvoiceID)
	}
	if in.Category != "" {
		where(` AND category = $%d`, in.Category)
	}
	if in.Cardholder != "" {
		where(` AND cardholder = $%d`, in.Cardholder)
	}
	if in.DescriptionContains != "" {
		where(` AND description ILIKE $%d`, "%"+in.DescriptionContains+"%")
	}
	if in.MinAmount > 0 {
		where(` AND amount >= $%d`, in.MinAmount)
	}
	if in.MaxAmount > 0 {
		where(` AND amount <= $%d`, in.MaxAmount)
	}
	if in.SharedOnly {
		q += ` AND is_shared = true`
	}

	// One extra row tells us whether the result was cut short.
	q += fmt.Sprintf(` ORDER BY amount DESC LIMIT %d`, limit+1)

	rows, err := t.db.Query(q, args...)
	if err != nil {
		return nil, searchOut{}, err
	}
	defer rows.Close()

	out := searchOut{Transactions: []txInfo{}, Currency: "NOK"}
	var b strings.Builder

	for rows.Next() {
		tx, err := scanTransaction(rows)
		if err != nil {
			return nil, searchOut{}, err
		}
		if len(out.Transactions) == limit {
			out.Truncated = true
			break
		}

		o := owes(tx.Amount, tx.Cardholder, tx.IsShared)
		out.Transactions = append(out.Transactions, txInfo{
			ID: tx.ID, InvoiceID: tx.InvoiceID, Date: tx.Date,
			Description: tx.Description, Amount: tx.Amount,
			Cardholder: tx.Cardholder, Category: tx.Category,
			IsShared: tx.IsShared, Owes: o,
		})
		out.Total += tx.Amount

		fmt.Fprintf(&b, "id=%-5d %s  %-34s %9.2f  %-22s %-9s %s\n",
			tx.ID, tx.Date, truncate(tx.Description, 34), tx.Amount,
			tx.Category, tx.Cardholder, sharedLabel(tx.IsShared))
	}
	if err := rows.Err(); err != nil {
		return nil, searchOut{}, err
	}

	out.Count = len(out.Transactions)
	out.Total = round2(out.Total)

	if out.Count == 0 {
		b.WriteString("No transactions matched those filters.")
	} else {
		fmt.Fprintf(&b, "\n%d transactions, %.2f NOK total", out.Count, out.Total)
		if out.Truncated {
			fmt.Fprintf(&b, " (showing the %d largest; narrow the filters to see the rest)", limit)
		}
	}

	return text(b.String()), out, nil
}

// ── list_categories ────────────────────────────────────────────────────────────

type ruleInfo struct {
	RuleID        int64  `json:"rule_id"`
	Pattern       string `json:"pattern"`
	Category      string `json:"category"`
	SplitOverride string `json:"split_override,omitempty"`
}

type categoriesOut struct {
	Categories       []string           `json:"categories"`
	SharedCategories []string           `json:"shared_categories"`
	Cardholders      []string           `json:"cardholders"`
	SplitRatio       map[string]float64 `json:"split_ratio"`
	CustomRules      []ruleInfo         `json:"custom_rules"`
	CustomCategories []Category         `json:"custom_categories"`
}

func (t *mcpTools) listCategories(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (
	*mcp.CallToolResult, categoriesOut, error,
) {
	ratio := map[string]float64{}
	for _, p := range SplitRatio {
		ratio[p.Name] = p.Ratio
	}

	out := categoriesOut{
		Categories:       AllCategories(),
		SharedCategories: SharedCategoryList(),
		Cardholders:      CardholderNames(),
		SplitRatio:       ratio,
		CustomRules:      []ruleInfo{},
		CustomCategories: currentCustomCategories(),
	}
	if out.CustomCategories == nil {
		out.CustomCategories = []Category{}
	}
	for _, r := range currentRules() {
		info := ruleInfo{RuleID: r.ID, Pattern: r.Pattern, Category: r.Category}
		if r.IsShared != nil {
			info.SplitOverride = sharedLabel(*r.IsShared)
		}
		out.CustomRules = append(out.CustomRules, info)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Categories: %s\n\n", strings.Join(AllCategories(), ", "))
	fmt.Fprintf(&b, "Shared by default: %s\n\n", strings.Join(SharedCategoryList(), ", "))
	b.WriteString("Shared transactions are split ")
	for i, p := range SplitRatio {
		if i > 0 {
			b.WriteString(" / ")
		}
		fmt.Fprintf(&b, "%s %.0f%%", p.Name, p.Ratio*100)
	}

	if len(out.CustomRules) > 0 {
		b.WriteString("\n\nSaved rules, checked before the built-in ones:\n")
		for _, r := range out.CustomRules {
			fmt.Fprintf(&b, "  rule_id=%-4d %-30s → %-24s %s\n",
				r.RuleID, r.Pattern, r.Category, r.SplitOverride)
		}
	}

	return text(b.String()), out, nil
}

// ── helpers ────────────────────────────────────────────────────────────────────

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}
}

func sharedLabel(shared bool) string {
	if shared {
		return "shared"
	}
	return "personal"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
