package main

import (
	"database/sql"
	"time"
)

// Table names match the SQLAlchemy models so an existing database keeps working.
// "transaction" is a reserved word in Postgres, so it is always quoted.
// pdf_data now holds the raw CSV bytes; the column name is kept for compatibility.
const schema = `
CREATE TABLE IF NOT EXISTS invoice (
    id          SERIAL PRIMARY KEY,
    filename    TEXT NOT NULL,
    uploaded_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    pdf_data    BYTEA NOT NULL
);

CREATE TABLE IF NOT EXISTS "transaction" (
    id              SERIAL PRIMARY KEY,
    invoice_id      INTEGER NOT NULL REFERENCES invoice(id),
    "date"          TEXT NOT NULL,
    description     TEXT NOT NULL,
    amount          DOUBLE PRECISION NOT NULL,
    cardholder      TEXT NOT NULL,
    category        TEXT NOT NULL,
    is_shared       BOOLEAN NOT NULL DEFAULT FALSE,
    orig_category   TEXT NOT NULL,
    orig_is_shared  BOOLEAN NOT NULL,
    orig_cardholder TEXT NOT NULL,
    modified        BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS ix_transaction_invoice_id ON "transaction" (invoice_id);

-- Category rules learned after the fact. These take precedence over the rules
-- compiled into parse.go, and are applied to future uploads automatically.
CREATE TABLE IF NOT EXISTS category_rule (
    id         SERIAL PRIMARY KEY,
    pattern    TEXT NOT NULL UNIQUE,
    category   TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

-- A rule can override the split instead of inheriting it from the category.
ALTER TABLE category_rule ADD COLUMN IF NOT EXISTS is_shared BOOLEAN;

-- Categories created after the fact, beyond the ones compiled into parse.go.
CREATE TABLE IF NOT EXISTS category (
    name          TEXT PRIMARY KEY,
    shared_default BOOLEAN NOT NULL DEFAULT FALSE,
    note          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

-- split_locked marks a split that was decided deliberately, so bulk rule runs
-- leave it alone. split_reason records why.
ALTER TABLE "transaction" ADD COLUMN IF NOT EXISTS split_locked BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE "transaction" ADD COLUMN IF NOT EXISTS split_reason TEXT NOT NULL DEFAULT '';

-- Location from the CSV, useful context for deciding whether a charge was joint.
ALTER TABLE "transaction" ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT '';
ALTER TABLE "transaction" ADD COLUMN IF NOT EXISTS country TEXT NOT NULL DEFAULT '';
`

type Invoice struct {
	ID         int64
	Filename   string
	UploadedAt time.Time
	Data       []byte
}

// InvoiceRow is an invoice plus its transaction count, without the file bytes.
type InvoiceRow struct {
	ID               int64
	Filename         string
	UploadedAt       time.Time
	TransactionCount int
}

func listInvoiceRows(db *sql.DB) ([]InvoiceRow, error) {
	rows, err := db.Query(`
		SELECT i.id, i.filename, i.uploaded_at, COUNT(t.id)
		FROM invoice i
		LEFT JOIN "transaction" t ON t.invoice_id = i.id
		GROUP BY i.id, i.filename, i.uploaded_at
		ORDER BY i.uploaded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []InvoiceRow{}
	for rows.Next() {
		var r InvoiceRow
		if err := rows.Scan(&r.ID, &r.Filename, &r.UploadedAt, &r.TransactionCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Transaction struct {
	ID             int64
	InvoiceID      int64
	Date           string
	Description    string
	Amount         float64
	Cardholder     string
	Category       string
	IsShared       bool
	OrigCategory   string
	OrigIsShared   bool
	OrigCardholder string
	Modified       bool
	SplitLocked    bool
	SplitReason    string
	City           string
	Country        string
}

const txColumns = `id, invoice_id, "date", description, amount, cardholder,
                   category, is_shared, orig_category, orig_is_shared,
                   orig_cardholder, modified, split_locked, split_reason,
                   city, country`

func scanTransaction(s interface{ Scan(...any) error }) (Transaction, error) {
	var t Transaction
	err := s.Scan(&t.ID, &t.InvoiceID, &t.Date, &t.Description, &t.Amount,
		&t.Cardholder, &t.Category, &t.IsShared, &t.OrigCategory,
		&t.OrigIsShared, &t.OrigCardholder, &t.Modified,
		&t.SplitLocked, &t.SplitReason, &t.City, &t.Country)
	return t, err
}

// getInvoice returns nil (and no error) when the invoice does not exist.
func getInvoice(db *sql.DB, id int64) (*Invoice, error) {
	var inv Invoice
	err := db.QueryRow(
		`SELECT id, filename, uploaded_at, pdf_data FROM invoice WHERE id = $1`, id,
	).Scan(&inv.ID, &inv.Filename, &inv.UploadedAt, &inv.Data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// invoiceExists avoids pulling the file bytes when we only need the row.
func invoiceExists(db *sql.DB, id int64) (bool, string, error) {
	var filename string
	err := db.QueryRow(`SELECT filename FROM invoice WHERE id = $1`, id).Scan(&filename)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, filename, nil
}

// listTransactions returns an invoice's rows ordered by id, which is insertion
// order and therefore already sorted by (date, cardholder) from parse time.
func listTransactions(db *sql.DB, invoiceID int64) ([]Transaction, error) {
	rows, err := db.Query(
		`SELECT `+txColumns+` FROM "transaction" WHERE invoice_id = $1 ORDER BY id`,
		invoiceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func getTransaction(db *sql.DB, id int64) (*Transaction, error) {
	row := db.QueryRow(`SELECT `+txColumns+` FROM "transaction" WHERE id = $1`, id)
	t, err := scanTransaction(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func insertInvoice(db *sql.DB, filename string, data []byte, parsed []ParsedTransaction) (*Invoice, error) {
	dbTransaction, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer dbTransaction.Rollback()

	var inv Invoice
	inv.Filename = filename
	err = dbTransaction.QueryRow(
		`INSERT INTO invoice (filename, uploaded_at, pdf_data)
		 VALUES ($1, $2, $3) RETURNING id, uploaded_at`,
		filename, time.Now().UTC(), data,
	).Scan(&inv.ID, &inv.UploadedAt)
	if err != nil {
		return nil, err
	}

	stmt, err := dbTransaction.Prepare(
		`INSERT INTO "transaction"
		 (invoice_id, "date", description, amount, cardholder, category, is_shared,
		  orig_category, orig_is_shared, orig_cardholder, modified, city, country)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,false,$11,$12)`,
	)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for _, p := range parsed {
		_, err := stmt.Exec(inv.ID, p.Date, p.Description, p.Amount, p.Cardholder,
			p.Category, p.IsShared, p.Category, p.IsShared, p.Cardholder,
			p.City, p.Country)
		if err != nil {
			return nil, err
		}
	}

	if err := dbTransaction.Commit(); err != nil {
		return nil, err
	}
	return &inv, nil
}

func deleteInvoice(db *sql.DB, id int64) (bool, error) {
	dbTransaction, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer dbTransaction.Rollback()

	// Delete children explicitly: the old SQLAlchemy cascade was ORM-level, so an
	// existing database may not have ON DELETE CASCADE on the foreign key.
	if _, err := dbTransaction.Exec(`DELETE FROM "transaction" WHERE invoice_id = $1`, id); err != nil {
		return false, err
	}

	res, err := dbTransaction.Exec(`DELETE FROM invoice WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	return true, dbTransaction.Commit()
}

func updateTransaction(db *sql.DB, t *Transaction) error {
	_, err := db.Exec(
		`UPDATE "transaction"
		 SET category = $1, is_shared = $2, cardholder = $3, modified = $4
		 WHERE id = $5`,
		t.Category, t.IsShared, t.Cardholder, t.Modified, t.ID,
	)
	return err
}

// updateTransactionFull also writes the split lock and its reason.
func updateTransactionFull(db *sql.DB, t *Transaction) error {
	_, err := db.Exec(
		`UPDATE "transaction"
		 SET category = $1, is_shared = $2, cardholder = $3, modified = $4,
		     split_locked = $5, split_reason = $6
		 WHERE id = $7`,
		t.Category, t.IsShared, t.Cardholder, t.Modified,
		t.SplitLocked, t.SplitReason, t.ID,
	)
	return err
}
