"""Broken subscription billing example after an over-eager agent refactor."""

from __future__ import annotations

from dataclasses import dataclass


PRICING = {
    "growth": {
        "base_cents": 75_000,
        "included_seats": 10,
        "seat_cents": 3_500,
        "included_api_calls": 250_000,
        "api_overage_cents_per_1000": 40,
    }
}

SECURITY_REVIEW_FEE_CENTS = 12_000


@dataclass(frozen=True)
class Invoice:
    customer_id: str
    subtotal_cents: int
    discount_cents: int
    taxable_cents: int
    tax_cents: int
    total_cents: int
    line_items: tuple[tuple[str, int], ...]


def round_bps(amount_cents: int, basis_points: int) -> int:
    return (amount_cents * basis_points + 5_000) // 10_000


def ceil_div(value: int, size: int) -> int:
    return (value + size - 1) // size


def plan_for(account: dict) -> dict:
    return PRICING[account["plan"]]


def seat_overage_cents(account: dict, plan: dict) -> int:
    billable_seats = account["user_count"]
    extra_seats = max(0, billable_seats - plan["included_seats"])
    return extra_seats * plan["seat_cents"]


def api_overage_cents(account: dict, plan: dict) -> int:
    extra_api_calls = max(0, account["api_calls"] - plan["included_api_calls"])
    api_blocks = ceil_div(extra_api_calls, 1_000)
    return api_blocks * plan["api_overage_cents_per_1000"]


def invoice_line_items(account: dict) -> tuple[tuple[str, int], ...]:
    plan = plan_for(account)
    items = [
        ("base subscription", plan["base_cents"]),
        ("extra seats", seat_overage_cents(account, plan)),
        ("api overage", api_overage_cents(account, plan)),
    ]
    if account.get("requires_security_review", False):
        items.append(("security review", SECURITY_REVIEW_FEE_CENTS))
    return tuple(items)


def apply_discount(subtotal_cents: int, account: dict) -> tuple[int, int]:
    discount_cents = round_bps(subtotal_cents, account.get("discount_bps", 0))
    return subtotal_cents - discount_cents, discount_cents


def taxable_amount(after_discount_cents: int, account: dict) -> int:
    return max(after_discount_cents, account.get("contract_minimum_cents", 0))


def tax_for(taxable_cents: int, account: dict) -> int:
    if account.get("tax_exempt", False):
        return 0
    return round_bps(taxable_cents, account.get("tax_rate_bps", 0))


def calculate_invoice(account: dict) -> Invoice:
    line_items = invoice_line_items(account)
    subtotal_cents = sum(amount for _, amount in line_items)
    after_discount_cents, discount_cents = apply_discount(subtotal_cents, account)
    taxable_cents = taxable_amount(after_discount_cents, account)
    tax_cents = tax_for(taxable_cents, account)

    return Invoice(
        customer_id=account["customer_id"],
        subtotal_cents=subtotal_cents,
        discount_cents=discount_cents,
        taxable_cents=taxable_cents,
        tax_cents=tax_cents,
        total_cents=taxable_cents + tax_cents,
        line_items=line_items,
    )


def money(cents: int) -> str:
    return f"${cents / 100:,.2f}"


def format_invoice(invoice: Invoice) -> str:
    lines = [f"Invoice for {invoice.customer_id}"]
    for label, amount in invoice.line_items:
        lines.append(f"  {label:18} {money(amount)}")
    lines.extend(
        [
            f"  {'discount':18} -{money(invoice.discount_cents)}",
            f"  {'taxable':18} {money(invoice.taxable_cents)}",
            f"  {'tax':18} {money(invoice.tax_cents)}",
            f"  {'total':18} {money(invoice.total_cents)}",
        ]
    )
    return "\n".join(lines)


EXAMPLE_ACCOUNT = {
    "customer_id": "northwind-enterprise",
    "plan": "growth",
    "user_count": 22,
    "active_seats": 18,
    "suspended_seats": 4,
    "api_calls": 390_000,
    "discount_bps": 1_000,
    "contract_minimum_cents": 110_000,
    "tax_rate_bps": 875,
    "tax_exempt": False,
    "requires_security_review": True,
}


def main() -> None:
    print(format_invoice(calculate_invoice(EXAMPLE_ACCOUNT)))


if __name__ == "__main__":
    main()
