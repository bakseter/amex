package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// corsMiddleware allows the dev frontend to call the API from another origin.
// Set CORS_ORIGINS to a comma-separated list, or "*" to allow everything.
func corsMiddleware(origins string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed["*"] || allowed[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			// So the frontend can read the download filename.
			c.Header("Access-Control-Expose-Headers", "Content-Disposition")
			c.Header("Access-Control-Max-Age", "86400")
		}

		// Answer preflight before it reaches the router.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dsn := env("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/invoices")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("could not open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		slog.Error("could not reach database", "err", err)
		os.Exit(1)
	}
	if _, err := db.Exec(schema); err != nil {
		slog.Error("could not create schema", "err", err)
		os.Exit(1)
	}
	if err := loadCategories(db); err != nil {
		slog.Error("could not load categories", "err", err)
		os.Exit(1)
	}
	if err := loadCategoryRules(db); err != nil {
		slog.Error("could not load category rules", "err", err)
		os.Exit(1)
	}

	s := &Server{db: db}

	r := gin.Default()
	r.Use(corsMiddleware(env("CORS_ORIGINS", "http://localhost:5173")))

	// For OpenTelemetry, add otelgin here:
	//   r.Use(otelgin.Middleware("amex-backend"))
	// from go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin

	api := r.Group("/api")
	{
		api.POST("/invoices", s.uploadInvoice)
		api.GET("/invoices", s.listInvoices)

		// Old path, kept so the frontend does not have to change; it now
		// returns the stored CSV rather than a PDF.
		api.GET("/invoices/:invoice_id/pdf", s.downloadInvoiceFile)
		api.GET("/invoices/:invoice_id/file", s.downloadInvoiceFile)
		api.GET("/invoices/:invoice_id/csv", s.downloadInvoiceFile)

		api.DELETE("/invoices/:invoice_id", s.deleteInvoice)

		api.GET("/invoices/:invoice_id/transactions", s.getTransactions)
		api.PATCH("/transactions/:transaction_id", s.updateTransaction)

		api.GET("/invoices/:invoice_id/summary", s.getSummary)

		api.POST("/invoices/:invoice_id/export", s.exportRedirect)
		api.GET("/invoices/:invoice_id/export/transactions", s.exportTransactions)
		api.GET("/invoices/:invoice_id/export/summary", s.exportSummary)

		api.GET("/meta", s.getMeta)
	}

	// MCP endpoint for kagent and other MCP clients.
	if env("MCP_ENABLED", "true") == "true" {
		path := env("MCP_PATH", "/mcp")
		mountMCP(r, db, path)
		slog.Info("MCP endpoint enabled", "path", path)
	}

	addr := ":" + env("PORT", "8000")
	slog.Info("listening", "addr", addr)
	if err := r.Run(addr); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
