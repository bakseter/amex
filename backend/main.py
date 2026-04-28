from __future__ import annotations

import csv
import io
import os
import tempfile
from pathlib import Path

import logging
import uvicorn
from fastapi import FastAPI, HTTPException, UploadFile, File
from fastapi.responses import StreamingResponse
from pydantic import BaseModel
from sqlalchemy import (
    create_engine,
    Column,
    Integer,
    String,
    Float,
    Boolean,
    LargeBinary,
    ForeignKey,
    DateTime,
    Text,
)
from sqlalchemy.orm import declarative_base, sessionmaker, relationship
from datetime import datetime, timezone
from opentelemetry import _logs
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import OTLPLogExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

from parse import (
    parse_invoice,
    ALL_CATEGORIES,
    CARDHOLDERS,
    SPLIT_RATIO,
    SHARED_CATEGORIES,
)

# ── Database setup ─────────────────────────────────────────────────────────────

DATABASE_URL = os.environ.get(
    "DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/invoices"
)

engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(bind=engine)
Base = declarative_base()


class InvoiceModel(Base):
    __tablename__ = "invoice"

    id = Column(Integer, primary_key=True, index=True)
    filename = Column(String, nullable=False)
    uploaded_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))
    pdf_data = Column(LargeBinary, nullable=False)

    transactions = relationship(
        "TransactionModel", back_populates="invoice", cascade="all, delete-orphan"
    )


class TransactionModel(Base):
    __tablename__ = "transaction"

    id = Column(Integer, primary_key=True, index=True)
    invoice_id = Column(Integer, ForeignKey("invoice.id"), nullable=False, index=True)

    # parsed values
    date = Column(String, nullable=False)
    description = Column(Text, nullable=False)
    amount = Column(Float, nullable=False)
    cardholder = Column(String, nullable=False)
    category = Column(String, nullable=False)
    is_shared = Column(Boolean, nullable=False, default=False)

    # original values (set once at parse time, never updated)
    orig_category = Column(String, nullable=False)
    orig_is_shared = Column(Boolean, nullable=False)
    orig_cardholder = Column(String, nullable=False)

    # track whether the row was manually edited
    modified = Column(Boolean, nullable=False, default=False)

    invoice = relationship("InvoiceModel", back_populates="transactions")


Base.metadata.create_all(bind=engine)

# ── FastAPI app ────────────────────────────────────────────────────────────────

app = FastAPI(title="Invoice Parser API")

# --─ OpenTelemetry setup ────────────────────────────────────────────────────────

resource = Resource.create({"service.name": "amex-backend"})
logger_provider = LoggerProvider(resource=resource)
_logs.set_logger_provider(logger_provider)

exporter = OTLPLogExporter()
logger_provider.add_log_record_processor(BatchLogRecordProcessor(exporter))

handler = LoggingHandler(level=logging.NOTSET, logger_provider=logger_provider)
logging.getLogger().addHandler(handler)

FastAPIInstrumentor.instrument_app(app)


# ── Helpers ────────────────────────────────────────────────────────────────────


def _owes(amount: float, cardholder: str, is_shared: bool) -> dict[str, float]:
    result = {}
    for person, ratio in SPLIT_RATIO.items():
        if is_shared:
            result[person] = round(amount * ratio, 2)
        else:
            result[person] = amount if cardholder == person else 0.0
    return result


def _tx_to_dict(tx: TransactionModel) -> dict:
    owes = _owes(tx.amount, tx.cardholder, tx.is_shared)
    return {
        "id": tx.id,
        "invoice_id": tx.invoice_id,
        "date": tx.date,
        "description": tx.description,
        "amount": tx.amount,
        "cardholder": tx.cardholder,
        "category": tx.category,
        "isShared": tx.is_shared,
        "owes": owes,
        "modified": tx.modified,
        "original": {
            "category": tx.orig_category,
            "isShared": tx.orig_is_shared,
            "cardholder": tx.orig_cardholder,
        },
    }


# ── Pydantic schemas ───────────────────────────────────────────────────────────


class TransactionUpdate(BaseModel):
    isShared: bool | None = None
    category: str | None = None
    cardholder: str | None = None


# ── Invoice endpoints ──────────────────────────────────────────────────────────


@app.post("/api/invoices", status_code=201)
def upload_invoice(file: UploadFile = File(...)):
    """Upload a PDF invoice, parse it, and store everything in Postgres."""
    if not file.filename or not file.filename.lower().endswith(".pdf"):
        raise HTTPException(status_code=400, detail="Only PDF files are accepted.")

    pdf_bytes = file.file.read()

    # Write to a temp file so pdfplumber can open it
    with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as tmp:
        tmp.write(pdf_bytes)
        tmp_path = tmp.name

    try:
        parsed = parse_invoice(tmp_path)
    finally:
        Path(tmp_path).unlink(missing_ok=True)

    with SessionLocal() as db:
        invoice = InvoiceModel(
            filename=file.filename,
            pdf_data=pdf_bytes,
        )
        db.add(invoice)
        db.flush()  # get invoice.id before committing

        for tx in parsed:
            row = TransactionModel(
                invoice_id=invoice.id,
                date=tx.date,
                description=tx.description,
                amount=tx.amount,
                cardholder=tx.cardholder,
                category=tx.category,
                is_shared=tx.is_shared,
                orig_category=tx.category,
                orig_is_shared=tx.is_shared,
                orig_cardholder=tx.cardholder,
                modified=False,
            )
            db.add(row)

        db.commit()
        db.refresh(invoice)

        return {
            "invoice_id": invoice.id,
            "filename": invoice.filename,
            "uploaded_at": invoice.uploaded_at.isoformat(),
            "transaction_count": len(parsed),
        }


@app.get("/api/invoices")
def list_invoices():
    with SessionLocal() as db:
        invoices = (
            db.query(InvoiceModel).order_by(InvoiceModel.uploaded_at.desc()).all()
        )
        return [
            {
                "id": inv.id,
                "filename": inv.filename,
                "uploaded_at": inv.uploaded_at.isoformat(),
                "transaction_count": len(inv.transactions),
            }
            for inv in invoices
        ]


@app.get("/api/invoices/{invoice_id}/pdf")
def download_invoice_pdf(invoice_id: int):
    """Download the original PDF stored for an invoice."""
    with SessionLocal() as db:
        inv = db.get(InvoiceModel, invoice_id)
        if inv is None:
            raise HTTPException(status_code=404, detail="Invoice not found")
        return StreamingResponse(
            io.BytesIO(inv.pdf_data),
            media_type="application/pdf",
            headers={"Content-Disposition": f'attachment; filename="{inv.filename}"'},
        )


@app.delete("/api/invoices/{invoice_id}", status_code=204)
def delete_invoice(invoice_id: int):
    with SessionLocal() as db:
        inv = db.get(InvoiceModel, invoice_id)
        if inv is None:
            raise HTTPException(status_code=404, detail="Invoice not found")
        db.delete(inv)
        db.commit()


# ── Transaction endpoints ──────────────────────────────────────────────────────


@app.get("/api/invoices/{invoice_id}/transactions")
def get_transactions(invoice_id: int):
    with SessionLocal() as db:
        inv = db.get(InvoiceModel, invoice_id)
        if inv is None:
            raise HTTPException(status_code=404, detail="Invoice not found")
        return [_tx_to_dict(tx) for tx in inv.transactions]


@app.patch("/api/transactions/{transaction_id}")
def update_transaction(transaction_id: int, update: TransactionUpdate):
    with SessionLocal() as db:
        tx = db.get(TransactionModel, transaction_id)
        if tx is None:
            raise HTTPException(status_code=404, detail="Transaction not found")

        changed = False

        if update.isShared is not None:
            tx.is_shared = update.isShared
            changed = True

        if update.category is not None:
            tx.category = update.category
            changed = True
            if update.isShared is None:
                tx.is_shared = update.category in SHARED_CATEGORIES

        if update.cardholder is not None:
            tx.cardholder = update.cardholder
            changed = True

        if changed:
            # Mark as modified only if current values differ from originals
            tx.modified = (
                tx.category != tx.orig_category
                or tx.is_shared != tx.orig_is_shared
                or tx.cardholder != tx.orig_cardholder
            )

        db.commit()
        db.refresh(tx)
        return _tx_to_dict(tx)


# ── Summary endpoint ───────────────────────────────────────────────────────────


@app.get("/api/invoices/{invoice_id}/summary")
def get_summary(invoice_id: int):
    with SessionLocal() as db:
        inv = db.get(InvoiceModel, invoice_id)
        if inv is None:
            raise HTTPException(status_code=404, detail="Invoice not found")

        persons = list(SPLIT_RATIO.keys())
        cats: dict[str, dict] = {}

        for tx in inv.transactions:
            owes = _owes(tx.amount, tx.cardholder, tx.is_shared)
            e = cats.setdefault(
                tx.category, {"total": 0.0, **{p: 0.0 for p in persons}}
            )
            e["total"] += tx.amount
            for p in persons:
                e[p] += owes[p]

        person_totals = {
            p: sum(
                _owes(tx.amount, tx.cardholder, tx.is_shared)[p]
                for tx in inv.transactions
            )
            for p in persons
        }
        grand = sum(tx.amount for tx in inv.transactions)

        return {
            "byCategory": sorted(cats.items(), key=lambda x: -x[1]["total"]),
            "personTotals": person_totals,
            "grandTotal": grand,
            "persons": persons,
        }


# ── Export endpoint ────────────────────────────────────────────────────────────


@app.post("/api/invoices/{invoice_id}/export")
def export_csv(invoice_id: int):
    """Return a ZIP-like response — actually two CSVs in a streaming multipart.
    For simplicity we return JSON with two base64-encoded CSVs, or just stream
    the transaction CSV. Clients can call /export/transactions and /export/summary."""
    raise HTTPException(
        status_code=301,
        detail="Use /api/invoices/{id}/export/transactions or /export/summary",
    )


@app.get("/api/invoices/{invoice_id}/export/transactions")
def export_transactions_csv(invoice_id: int):
    with SessionLocal() as db:
        inv = db.get(InvoiceModel, invoice_id)
        if inv is None:
            raise HTTPException(status_code=404, detail="Invoice not found")

        persons = list(SPLIT_RATIO.keys())
        buf = io.StringIO()
        w = csv.writer(buf)
        w.writerow(
            [
                "Date",
                "Description",
                "Category",
                "Amount (NOK)",
                "Cardholder",
                "Shared",
                "Modified",
            ]
            + [f"{p} owes (NOK)" for p in persons]
        )
        for tx in sorted(inv.transactions, key=lambda t: (t.date, t.cardholder)):
            owes = _owes(tx.amount, tx.cardholder, tx.is_shared)
            w.writerow(
                [
                    tx.date,
                    tx.description,
                    tx.category,
                    f"{tx.amount:.2f}",
                    tx.cardholder,
                    "yes" if tx.is_shared else "no",
                    "yes" if tx.modified else "no",
                    *[f"{owes[p]:.2f}" for p in persons],
                ]
            )

        stem = Path(inv.filename).stem
        buf.seek(0)
        return StreamingResponse(
            iter([buf.getvalue()]),
            media_type="text/csv",
            headers={"Content-Disposition": f'attachment; filename="{stem}.csv"'},
        )


@app.get("/api/invoices/{invoice_id}/export/summary")
def export_summary_csv(invoice_id: int):
    with SessionLocal() as db:
        inv = db.get(InvoiceModel, invoice_id)
        if inv is None:
            raise HTTPException(status_code=404, detail="Invoice not found")

        persons = list(SPLIT_RATIO.keys())
        cats: dict[str, dict] = {}

        for tx in inv.transactions:
            owes = _owes(tx.amount, tx.cardholder, tx.is_shared)
            e = cats.setdefault(
                tx.category, {"total": 0.0, **{p: 0.0 for p in persons}}
            )
            e["total"] += tx.amount
            for p in persons:
                e[p] += owes[p]

        buf = io.StringIO()
        w = csv.writer(buf)
        w.writerow(["Category", "Total (NOK)"] + [f"{p} owes (NOK)" for p in persons])
        for cat, vals in sorted(cats.items(), key=lambda x: -x[1]["total"]):
            w.writerow(
                [cat, f"{vals['total']:.2f}"] + [f"{vals[p]:.2f}" for p in persons]
            )

        grand = sum(tx.amount for tx in inv.transactions)
        person_totals = {
            p: sum(
                _owes(tx.amount, tx.cardholder, tx.is_shared)[p]
                for tx in inv.transactions
            )
            for p in persons
        }
        w.writerow(
            ["TOTAL", f"{grand:.2f}"] + [f"{person_totals[p]:.2f}" for p in persons]
        )

        stem = Path(inv.filename).stem
        buf.seek(0)
        return StreamingResponse(
            iter([buf.getvalue()]),
            media_type="text/csv",
            headers={
                "Content-Disposition": f'attachment; filename="{stem}-summary.csv"'
            },
        )


# ── Meta endpoint ──────────────────────────────────────────────────────────────


@app.get("/api/meta")
def get_meta():
    return {
        "categories": ALL_CATEGORIES,
        "cardholders": list(CARDHOLDERS.values()),
        "sharedCategories": list(SHARED_CATEGORIES),
    }


# ── Entrypoint ─────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print("\n  Open http://localhost:8000/docs\n")
    uvicorn.run(app, host="0.0.0.0", port=8000, log_level="info")
