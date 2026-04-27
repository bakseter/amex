from __future__ import annotations

import csv
import sys
from pathlib import Path

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from parse import (
    Transaction,
    parse_invoice,
    ALL_CATEGORIES,
    CARDHOLDERS,
    SPLIT_RATIO,
    SHARED_CATEGORIES,
)

pdf_path: str = ""
transactions: list[Transaction] = []

app = FastAPI()


class TransactionUpdate(BaseModel):
    isShared: bool | None = None
    category: str | None = None
    cardholder: str | None = None


@app.get("/api/transactions")
def get_transactions():
    return [transaction.to_dict() for transaction in transactions]


@app.get("/api/meta")
def get_meta():
    return {
        "categories": ALL_CATEGORIES,
        "cardholders": list(CARDHOLDERS.values()),
        "sharedCategories": list(SHARED_CATEGORIES),
        "pdf": pdf_path,
    }


@app.patch("/api/transactions/{transaction_id}")
def update_transaction(transaction_id: int, update: TransactionUpdate):
    transaction = next((t for t in transactions if t.id == transaction_id), None)

    if transaction is None:
        return JSONResponse(status_code=404, content={"error": "not found"})

    if update.isShared is not None:
        transaction.is_shared = update.isShared

    if update.category is not None:
        transaction.category = update.category
        if update.isShared is None:
            transaction.is_shared = update.category in SHARED_CATEGORIES

    if update.cardholder is not None:
        transaction.cardholder = update.cardholder

    return transaction.to_dict()


@app.get("/api/summary")
def get_summary():
    persons = list(SPLIT_RATIO.keys())
    cats: dict[str, dict] = {}

    for transaction in transactions:
        owes = transaction.owes()
        e = cats.setdefault(
            transaction.category, {"total": 0.0, **{p: 0.0 for p in persons}}
        )

        e["total"] += transaction.amount

        for p in persons:
            e[p] += owes[p]

    person_totals = {
        p: sum(transaction.owes()[p] for transaction in transactions) for p in persons
    }

    grand = sum(transaction.amount for transaction in transactions)

    return {
        "byCategory": sorted(cats.items(), key=lambda x: -x[1]["total"]),
        "personTotals": person_totals,
        "grandTotal": grand,
        "persons": persons,
    }


@app.post("/api/export")
def export_csv():
    stem = Path(pdf_path).stem
    persons = list(SPLIT_RATIO.keys())

    transaction_path = stem + ".csv"

    with open(transaction_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(
            ["Date", "Description", "Category", "Amount (NOK)", "Cardholder", "Shared"]
            + [f"{p} owes (NOK)" for p in persons]
        )

        for transaction in transactions:
            owes = transaction.owes()

            w.writerow(
                [
                    transaction.date,
                    transaction.description,
                    transaction.category,
                    f"{transaction.amount:.2f}",
                    transaction.cardholder,
                    "yes" if transaction.is_shared else "no",
                    *[f"{owes[p]:.2f}" for p in persons],
                ]
            )

    summary_path = stem + "-summary.csv"
    cats: dict[str, dict] = {}

    for transaction in transactions:
        owes = transaction.owes()
        e = cats.setdefault(
            transaction.category, {"total": 0.0, **{p: 0.0 for p in persons}}
        )
        e["total"] += transaction.amount

        for p in persons:
            e[p] += owes[p]

    with open(summary_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["Category", "Total (NOK)"] + [f"{p} owes (NOK)" for p in persons])

        for cat, vals in sorted(cats.items(), key=lambda x: -x[1]["total"]):
            w.writerow(
                [cat, f"{vals['total']:.2f}"] + [f"{vals[p]:.2f}" for p in persons]
            )

        grand = sum(transaction.amount for transaction in transactions)

        w.writerow(
            ["TOTAL", f"{grand:.2f}"]
            + [
                f"{sum(transaction.owes()[p] for transaction in transactions):.2f}"
                for p in persons
            ]
        )

    return {"transaction_path": transaction_path, "summary_path": summary_path}


def main() -> None:
    global pdf_path, transactions
    args = sys.argv[1:]
    pdf_path = next((a for a in args if not a.startswith("--")), "invoice.pdf")

    print(f"\n  Parsing {pdf_path}...")
    transactions = parse_invoice(pdf_path)
    print(f"  Loaded {len(transactions)} transactions.")
    print("\n  Open http://localhost:8000\n")

    uvicorn.run(app, host="0.0.0.0", port=8000, log_level="warning")


if __name__ == "__main__":
    main()
