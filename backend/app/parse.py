from __future__ import annotations

import re
from collections import Counter
from dataclasses import dataclass

import pdfplumber


# ── Configuration ──────────────────────────────────────────────────────────────

CARDHOLDERS: dict[str, str] = {
    "Andreas": "Andreas",
    "Nikoline": "Nikoline",
}

SPLIT_RATIO: dict[str, float] = {
    "Andreas": 0.65,
    "Nikoline": 0.35,
}

CATEGORY_RULES: list[tuple[str, str]] = [
    (r"kiwi|meny|coop|joker|rema|bunnpris|extra", "Groceries"),
    (r"wolt|foodora|just ?eat|mcdonald|burger|pizza|sushi", "Takeaway"),
    (r"kaffebrenneriet|joe.the.juice|starbucks|espresso", "Café"),
    (r"restaurant|bistro|grill|mat og drikke|ochaya", "Dining out"),
    (r"ruterappen|ruter|atb|skyss|kolumbus|entur", "Public transport"),
    (r"bolt|uber|taxi|lyft|cabonline", "Taxi"),
    (r"wideroe|sas|norwegian|flyr|ryanair|lufthansa", "Flights"),
    (r"h.m|zara|mango|weekday|cos |arket|monki|nakd", "Clothing"),
    (r"elkjop|komplett|power |apple|samsung|kjell", "Electronics"),
    (r"clas ohlson|biltema|byggmax|jula", "Hardware / tools"),
    (r"nordicnest|ikea|jysk|bolia|hay |diy", "Home & interior"),
    (r"skolyx|zalando|boozt|footlocker", "Shoes"),
    (r"nikita|caia|lookfantastic|kicks", "Beauty & personal care"),
    (r"ticketmaster|billettservice|eventbrite", "Events / tickets"),
    (r"colosseum kino|sf kino|nordisk film kino", "Cinema"),
    (r"spotify|netflix|youtube|apple.com.bill|hbo|viaplay|disney", "Streaming"),
    (r"vinmonopolet", "Alcohol"),
    (r"jagex|steam|playstation|xbox|nintendo|kodekloud", "Games / learning"),
    (r"claude\.ai|anthropic|openai|chatgpt", "AI subscriptions"),
    (r"hetzner|digitalocean|aws|google cloud|azure|linode", "Cloud / hosting"),
    (r"linuxfoundation|udemy|coursera|pluralsight", "Education"),
    (r"paypal \*google|google\.com", "Google services"),
    (r"wikipedia", "Donations"),
    (r"apotek|vitusapotek|boots|farma", "Pharmacy"),
    (r"gym|trening|crossfit|sats|evo fitness", "Fitness"),
    (r"havferd", "Activities / experiences"),
    (r"rouleur|markedsplassen|oslo gate|lett ", "Other shopping"),
    (r"kunstnernes", "Culture"),
    (r"medlemsavgift", "Card membership fee"),
]

SHARED_CATEGORIES: set[str] = {
    "Groceries",
    "Takeaway",
    "Dining out",
    "Alcohol",
    "Activities / experiences",
    "Café",
    "Card membership fee",
}

ALL_CATEGORIES: list[str] = sorted(
    {cat for _, cat in CATEGORY_RULES} | {"Uncategorized"}
)

COLUMN_SPLIT_X = 260


# ── Data model ─────────────────────────────────────────────────────────────────


@dataclass
class Transaction:
    id: int
    date: str
    description: str
    amount: float
    cardholder: str
    category: str = "Uncategorized"
    is_shared: bool = False

    def owes(self) -> dict[str, float]:
        result = {}
        for person, ratio in SPLIT_RATIO.items():
            if self.is_shared:
                result[person] = round(self.amount * ratio, 2)
            else:
                result[person] = self.amount if self.cardholder == person else 0.0
        return result

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "date": self.date,
            "description": self.description,
            "amount": self.amount,
            "cardholder": self.cardholder,
            "category": self.category,
            "isShared": self.is_shared,
            "owes": self.owes(),
        }


# ── PDF parsing ────────────────────────────────────────────────────────────────


def _nok(s: str) -> float:
    return float(s.replace(".", "").replace(",", "."))


def _categorize(description: str) -> str:
    desc = description.lower()

    for pattern, category in CATEGORY_RULES:
        if re.search(pattern, desc):
            return category

    return "Uncategorized"


_TX_LINE = re.compile(r"^(\d{2}\.\d{2}\.\d{2})\s+(.+?)\s+([\d.]+,\d{2})$")
_SKIP = re.compile(
    r"totalt|sum |betaling mottatt|godskrevet|transaksjons|dato|detaljer|"
    r"denne periode|beløp nok|utenlandsk|valutakurs|transaksjonsavgift|"
    r"kort som slutter|^-dato",
    re.IGNORECASE,
)
_HOLDER_HDR = re.compile(r"nye transaksjoner for\s*(andreas|nikoline)", re.IGNORECASE)
_STOP_SECTION = re.compile(
    r"andre kontotransaksjoner|gjeldende renter|eksempel på", re.IGNORECASE
)


def _col_lines(page, x_min: float, x_max: float) -> list[str]:
    words = [w for w in page.extract_words() if x_min <= w["x0"] < x_max]
    buckets: dict[int, list] = {}
    for w in words:
        key = round(w["top"] / 3) * 3
        buckets.setdefault(key, []).append(w)
    return [
        " ".join(w["text"] for w in sorted(v, key=lambda x: x["x0"]))
        for _, v in sorted(buckets.items())
    ]


def _parse_col(pages, x_min: float, x_max: float, start_holder: str) -> list[tuple]:
    txs: list[tuple] = []
    seen: Counter = Counter()
    current = start_holder
    stopped = False

    for page in pages:
        if stopped:
            break

        for line in _col_lines(page, x_min, x_max):
            if _STOP_SECTION.search(line):
                stopped = True
                break

            m_h = _HOLDER_HDR.search(line)
            if m_h:
                current = "Andreas" if "andreas" in m_h.group(1).lower() else "Nikoline"
                continue

            if _SKIP.search(line):
                continue

            m = _TX_LINE.match(line.strip())

            if m:
                amount = _nok(m.group(3))
                if amount <= 0:
                    continue

                base_key = (m.group(1), m.group(2).strip(), amount, current)
                seen[base_key] += 1
                txs.append(base_key + (seen[base_key],))
    return txs


def parse_invoice(pdf_path: str) -> list[Transaction]:
    default_holder = list(CARDHOLDERS.values())[0]

    with pdfplumber.open(pdf_path) as pdf:
        left = _parse_col(pdf.pages, 0, COLUMN_SPLIT_X, default_holder)
        right = _parse_col(pdf.pages, COLUMN_SPLIT_X, 9999, default_holder)

    transactions: list[Transaction] = []

    for idx, (date, desc, amount, holder, _) in enumerate(left + right):
        cat = _categorize(desc)
        transactions.append(
            Transaction(
                id=idx,
                date=date,
                description=desc,
                amount=amount,
                cardholder=holder,
                category=cat,
                is_shared=cat in SHARED_CATEGORIES,
            )
        )

    transactions.sort(key=lambda t: (t.date, t.cardholder))
    # Re-assign sequential IDs after sort
    for i, tx in enumerate(transactions):
        tx.id = i

    return transactions
