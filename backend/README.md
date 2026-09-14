# amex-backend

Reads Amex CSV exports.

## Run

```bash
cd amex-go
go mod tidy
DATABASE_URL="postgresql://postgres:postgres@localhost:5432/invoices" go run .
```

Env vars: `DATABASE_URL` (same default as the Python app), `PORT` (default 8000),
`CORS_ORIGINS` (comma-separated, default `http://localhost:5173`, `*` allows all).

Upload:

```bash
curl -F "file=@statement.csv" http://localhost:8000/api/invoices
```

## Endpoints

| Method | Path                                     | Notes                                           |
| ------ | ---------------------------------------- | ----------------------------------------------- |
| POST   | `/api/invoices`                          | multipart field `file`, now `.csv`              |
| GET    | `/api/invoices`                          |                                                 |
| GET    | `/api/invoices/{id}/pdf`                 | serves the stored CSV; kept for frontend compat |
| GET    | `/api/invoices/{id}/file`                | alias for the above                             |
| DELETE | `/api/invoices/{id}`                     | 204                                             |
| GET    | `/api/invoices/{id}/transactions`        |                                                 |
| PATCH  | `/api/transactions/{id}`                 | `{isShared, category, cardholder}`              |
| GET    | `/api/invoices/{id}/summary`             |                                                 |
| POST   | `/api/invoices/{id}/export`              | 301, same message as before                     |
| GET    | `/api/invoices/{id}/export/transactions` |                                                 |
| GET    | `/api/invoices/{id}/export/summary`      |                                                 |
| GET    | `/api/meta`                              |                                                 |

## Schema

Tables `invoice` and `"transaction"` keep the SQLAlchemy names and columns, so an
existing database works as-is. `pdf_data` now holds CSV bytes — renaming it is a
one-line change in `store.go` plus an `ALTER TABLE ... RENAME COLUMN`.

## MCP server

The same binary serves MCP over streamable HTTP at `/mcp`, talking to Postgres
directly rather than calling its own REST API. Disable with `MCP_ENABLED=false`,
move it with `MCP_PATH`.

Tools: `list_invoices`, `get_invoice_summary`, `search_transactions`,
`list_categories`, `list_uncategorized`, `preview_category_rules`,
`apply_category_rules`, `delete_category_rules`, `create_categories`,
`review_splits`, `set_splits`, `update_transactions`.

Categorization is rule-based rather than row-based. `list_uncategorized` groups
spend by merchant, `preview_category_rules` is a dry run, and
`apply_category_rules` saves the rules and applies them in one write. Saved rules
live in the `category_rule` table, are checked before the rules compiled into
`parse.go`, and apply to future uploads automatically.

Categories are extensible: the `category` table holds names created after the
fact, each with its own `shared_default`. Near-duplicate names ("Cafe" next to
"Café") are rejected at creation.

The shared/personal flag resolves in three layers — the category's default, then
a per-merchant override on a rule, then a per-transaction decision, which sets
`split_locked` so later rule runs leave it alone. `review_splits` ranks rows by
how much the balance moves if the flag flips, rather than by count.

Inspect them locally:

```bash
npx @modelcontextprotocol/inspector
# connect to http://localhost:8000/mcp, transport "Streamable HTTP"
```

## CSV assumptions

- Columns looked up by header name: `Dato`, `Beskrivelse`, `Beløp`, `Kortmedlem`.
  English headers (`Date`, `Description`, `Amount`, `Card Member`) also work.
- `Dato` is parsed as `MM/DD/YYYY`; output keeps the old `dd.mm.yy` format.
- `Beløp` handles `1.234,56` and the Unicode minus (U+2212) Amex writes.
- Rows with an amount <= 0 are dropped, matching the PDF parser. Flip
  `SkipNonPositive` in `parse.go` to keep credits.
- Amex's own `Kategori` column is ignored; the regex rules in `parse.go` decide.
