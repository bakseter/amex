package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Server struct {
	db *sql.DB
}

// isoTime matches what Python produced: the column is TIMESTAMP WITHOUT TIME
// ZONE, so datetime.isoformat() emitted no offset.
const isoTime = "2006-01-02T15:04:05.000000"

// ── Response shapes ────────────────────────────────────────────────────────────

type originalDTO struct {
	Category   string `json:"category"`
	IsShared   bool   `json:"isShared"`
	Cardholder string `json:"cardholder"`
}

type txDTO struct {
	ID          int64              `json:"id"`
	InvoiceID   int64              `json:"invoice_id"`
	Date        string             `json:"date"`
	Description string             `json:"description"`
	Amount      float64            `json:"amount"`
	Cardholder  string             `json:"cardholder"`
	Category    string             `json:"category"`
	IsShared    bool               `json:"isShared"`
	Owes        map[string]float64 `json:"owes"`
	Modified    bool               `json:"modified"`
	SplitLocked bool               `json:"splitLocked"`
	SplitReason string             `json:"splitReason"`
	City        string             `json:"city"`
	Country     string             `json:"country"`
	Original    originalDTO        `json:"original"`
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func owes(amount float64, cardholder string, isShared bool) map[string]float64 {
	res := make(map[string]float64, len(SplitRatio))
	for _, p := range SplitRatio {
		switch {
		case isShared:
			res[p.Name] = round2(amount * p.Ratio)
		case p.Name == cardholder:
			res[p.Name] = amount
		default:
			res[p.Name] = 0
		}
	}
	return res
}

func toDTO(t Tx) txDTO {
	return txDTO{
		ID:          t.ID,
		InvoiceID:   t.InvoiceID,
		Date:        t.Date,
		Description: t.Description,
		Amount:      t.Amount,
		Cardholder:  t.Cardholder,
		Category:    t.Category,
		IsShared:    t.IsShared,
		Owes:        owes(t.Amount, t.Cardholder, t.IsShared),
		Modified:    t.Modified,
		SplitLocked: t.SplitLocked,
		SplitReason: t.SplitReason,
		City:        t.City,
		Country:     t.Country,
		Original: originalDTO{
			Category:   t.OrigCategory,
			IsShared:   t.OrigIsShared,
			Cardholder: t.OrigCardholder,
		},
	}
}

// ── Error helpers ──────────────────────────────────────────────────────────────

// detail mirrors FastAPI's {"detail": "..."} error body.
func detail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"detail": msg})
}

func serverError(c *gin.Context, err error) {
	slog.Error("request failed", "path", c.FullPath(), "err", err)
	detail(c, http.StatusInternalServerError, "Internal server error")
}

func pathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		detail(c, http.StatusUnprocessableEntity, "Invalid id")
		return 0, false
	}
	return id, true
}

// loadInvoiceTransactions resolves the invoice and its rows, writing a 404 when
// the invoice does not exist.
func (s *Server) loadInvoiceTransactions(c *gin.Context) (string, []Tx, bool) {
	id, ok := pathID(c, "invoice_id")
	if !ok {
		return "", nil, false
	}
	exists, filename, err := invoiceExists(s.db, id)
	if err != nil {
		serverError(c, err)
		return "", nil, false
	}
	if !exists {
		detail(c, http.StatusNotFound, "Invoice not found")
		return "", nil, false
	}
	txs, err := listTransactions(s.db, id)
	if err != nil {
		serverError(c, err)
		return "", nil, false
	}
	return filename, txs, true
}

// ── Invoice endpoints ──────────────────────────────────────────────────────────

func (s *Server) uploadInvoice(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		detail(c, http.StatusUnprocessableEntity, `Expected a file upload in the "file" field.`)
		return
	}
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		detail(c, http.StatusBadRequest, "Only CSV files are accepted.")
		return
	}

	f, err := header.Open()
	if err != nil {
		serverError(c, err)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		serverError(c, err)
		return
	}

	parsed, err := ParseCSV(data)
	if err != nil {
		detail(c, http.StatusBadRequest, err.Error())
		return
	}

	inv, err := insertInvoice(s.db, header.Filename, data, parsed)
	if err != nil {
		serverError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"invoice_id":        inv.ID,
		"filename":          inv.Filename,
		"uploaded_at":       inv.UploadedAt.Format(isoTime),
		"transaction_count": len(parsed),
	})
}

func (s *Server) listInvoices(c *gin.Context) {
	rows, err := listInvoiceRows(s.db)
	if err != nil {
		serverError(c, err)
		return
	}

	out := []gin.H{}
	for _, r := range rows {
		out = append(out, gin.H{
			"id":                r.ID,
			"filename":          r.Filename,
			"uploaded_at":       r.UploadedAt.Format(isoTime),
			"transaction_count": r.TransactionCount,
		})
	}
	c.JSON(http.StatusOK, out)
}

// downloadInvoiceFile keeps the old /pdf path for frontend compatibility, but
// now serves the stored CSV. /file is an alias you can migrate to.
func (s *Server) downloadInvoiceFile(c *gin.Context) {
	id, ok := pathID(c, "invoice_id")
	if !ok {
		return
	}
	inv, err := getInvoice(s.db, id)
	if err != nil {
		serverError(c, err)
		return
	}
	if inv == nil {
		detail(c, http.StatusNotFound, "Invoice not found")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, inv.Filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", inv.Data)
}

func (s *Server) deleteInvoice(c *gin.Context) {
	id, ok := pathID(c, "invoice_id")
	if !ok {
		return
	}
	deleted, err := deleteInvoice(s.db, id)
	if err != nil {
		serverError(c, err)
		return
	}
	if !deleted {
		detail(c, http.StatusNotFound, "Invoice not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Transaction endpoints ──────────────────────────────────────────────────────

func (s *Server) getTransactions(c *gin.Context) {
	_, txs, ok := s.loadInvoiceTransactions(c)
	if !ok {
		return
	}
	out := make([]txDTO, 0, len(txs))
	for _, t := range txs {
		out = append(out, toDTO(t))
	}
	c.JSON(http.StatusOK, out)
}

type txUpdate struct {
	IsShared    *bool   `json:"isShared"`
	Category    *string `json:"category"`
	Cardholder  *string `json:"cardholder"`
	SplitReason *string `json:"splitReason"`
	ResetSplit  *bool   `json:"resetSplit"`
}

func (s *Server) updateTransaction(c *gin.Context) {
	id, ok := pathID(c, "transaction_id")
	if !ok {
		return
	}

	var body txUpdate
	if err := c.ShouldBindJSON(&body); err != nil {
		detail(c, http.StatusUnprocessableEntity, "Invalid request body")
		return
	}

	t, err := getTransaction(s.db, id)
	if err != nil {
		serverError(c, err)
		return
	}
	if t == nil {
		detail(c, http.StatusNotFound, "Transaction not found")
		return
	}

	changed := false

	if body.IsShared != nil {
		t.IsShared = *body.IsShared
		t.SplitLocked = true // a deliberate choice survives later rule runs
		changed = true
	}

	if body.SplitReason != nil {
		t.SplitReason = strings.TrimSpace(*body.SplitReason)
		changed = true
	}

	if body.ResetSplit != nil && *body.ResetSplit {
		t.SplitLocked = false
		t.SplitReason = ""
		t.IsShared = sharedDefault(t.Category)
		changed = true
	}

	if body.Category != nil {
		t.Category = *body.Category
		changed = true
		// Changing category alone re-derives the shared flag, same as before.
		if body.IsShared == nil {
			t.IsShared = sharedDefault(t.Category)
		}
	}

	if body.Cardholder != nil {
		t.Cardholder = *body.Cardholder
		changed = true
	}

	if changed {
		t.Modified = t.Category != t.OrigCategory ||
			t.IsShared != t.OrigIsShared ||
			t.Cardholder != t.OrigCardholder

		if err := updateTransactionFull(s.db, t); err != nil {
			serverError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, toDTO(*t))
}

// ── Summary ────────────────────────────────────────────────────────────────────

// categoryTotals aggregates per category, preserving first-appearance order so
// ties sort the same way Python's stable sort did.
func categoryTotals(txs []Tx) ([]string, map[string]map[string]float64) {
	persons := PersonNames()
	var order []string
	totals := map[string]map[string]float64{}

	for _, t := range txs {
		e, seen := totals[t.Category]
		if !seen {
			e = map[string]float64{"total": 0}
			for _, p := range persons {
				e[p] = 0
			}
			totals[t.Category] = e
			order = append(order, t.Category)
		}
		e["total"] += t.Amount
		for p, v := range owes(t.Amount, t.Cardholder, t.IsShared) {
			e[p] += v
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		return totals[order[i]]["total"] > totals[order[j]]["total"]
	})
	return order, totals
}

func personTotals(txs []Tx) map[string]float64 {
	out := map[string]float64{}
	for _, p := range PersonNames() {
		out[p] = 0
	}
	for _, t := range txs {
		for p, v := range owes(t.Amount, t.Cardholder, t.IsShared) {
			out[p] += v
		}
	}
	return out
}

func (s *Server) getSummary(c *gin.Context) {
	_, txs, ok := s.loadInvoiceTransactions(c)
	if !ok {
		return
	}

	order, totals := categoryTotals(txs)

	// byCategory is a list of [category, {total, <person>...}] pairs, matching
	// the shape Python's sorted(dict.items()) produced.
	byCategory := make([][]any, 0, len(order))
	for _, cat := range order {
		byCategory = append(byCategory, []any{cat, totals[cat]})
	}

	grand := 0.0
	for _, t := range txs {
		grand += t.Amount
	}

	c.JSON(http.StatusOK, gin.H{
		"byCategory":   byCategory,
		"personTotals": personTotals(txs),
		"grandTotal":   grand,
		"persons":      PersonNames(),
	})
}

// ── CSV export ─────────────────────────────────────────────────────────────────

func (s *Server) exportRedirect(c *gin.Context) {
	detail(c, http.StatusMovedPermanently,
		"Use /api/invoices/{id}/export/transactions or /export/summary")
}

func sendCSV(c *gin.Context, filename string, write func(*csv.Writer) error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = true // Python's csv.writer default

	if err := write(w); err != nil {
		serverError(c, err)
		return
	}
	w.Flush()
	if err := w.Error(); err != nil {
		serverError(c, err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func stem(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (s *Server) exportTransactions(c *gin.Context) {
	filename, txs, ok := s.loadInvoiceTransactions(c)
	if !ok {
		return
	}
	persons := PersonNames()

	sendCSV(c, stem(filename)+".csv", func(w *csv.Writer) error {
		head := []string{"Date", "Description", "Category", "Amount (NOK)",
			"Cardholder", "Shared", "Modified"}
		for _, p := range persons {
			head = append(head, p+" owes (NOK)")
		}
		if err := w.Write(head); err != nil {
			return err
		}

		for _, t := range txs {
			o := owes(t.Amount, t.Cardholder, t.IsShared)
			row := []string{
				t.Date,
				t.Description,
				t.Category,
				fmt.Sprintf("%.2f", t.Amount),
				t.Cardholder,
				yesNo(t.IsShared),
				yesNo(t.Modified),
			}
			for _, p := range persons {
				row = append(row, fmt.Sprintf("%.2f", o[p]))
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Server) exportSummary(c *gin.Context) {
	filename, txs, ok := s.loadInvoiceTransactions(c)
	if !ok {
		return
	}
	persons := PersonNames()
	order, totals := categoryTotals(txs)

	grand := 0.0
	for _, t := range txs {
		grand += t.Amount
	}
	pt := personTotals(txs)

	sendCSV(c, stem(filename)+"-summary.csv", func(w *csv.Writer) error {
		head := []string{"Category", "Total (NOK)"}
		for _, p := range persons {
			head = append(head, p+" owes (NOK)")
		}
		if err := w.Write(head); err != nil {
			return err
		}

		for _, cat := range order {
			row := []string{cat, fmt.Sprintf("%.2f", totals[cat]["total"])}
			for _, p := range persons {
				row = append(row, fmt.Sprintf("%.2f", totals[cat][p]))
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}

		row := []string{"TOTAL", fmt.Sprintf("%.2f", grand)}
		for _, p := range persons {
			row = append(row, fmt.Sprintf("%.2f", pt[p]))
		}
		return w.Write(row)
	})
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ── Meta ───────────────────────────────────────────────────────────────────────

func (s *Server) getMeta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"categories":       AllCategories(),
		"splitRatio":       splitRatioMap(),
		"cardholders":      CardholderNames(),
		"sharedCategories": SharedCategoryList(),
	})
}
