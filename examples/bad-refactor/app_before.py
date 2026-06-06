"""Working subscription billing example used by the bad-refactor walkthrough."""

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


def calculate_invoice(account: dict) -> Invoice:
    plan = PRICING[account["plan"]]

    billable_seats = account["active_seats"]
    extra_seats = max(0, billable_seats - plan["included_seats"])
    seat_overage_cents = extra_seats * plan["seat_cents"]

    extra_api_calls = max(0, account["api_calls"] - plan["included_api_calls"])
    api_blocks = ceil_div(extra_api_calls, 1_000)
    api_overage_cents = api_blocks * plan["api_overage_cents_per_1000"]

    line_items = [
        ("base subscription", plan["base_cents"]),
        ("extra seats", seat_overage_cents),
        ("api overage", api_overage_cents),
    ]
    if account.get("requires_security_review", False):
        line_items.append(("security review", SECURITY_REVIEW_FEE_CENTS))

    subtotal_cents = sum(amount for _, amount in line_items)
    discount_cents = round_bps(subtotal_cents, account.get("discount_bps", 0))
    after_discount_cents = subtotal_cents - discount_cents
    taxable_cents = max(after_discount_cents, account.get("contract_minimum_cents", 0))

    if account.get("tax_exempt", False):
        tax_cents = 0
    else:
        tax_cents = round_bps(taxable_cents, account.get("tax_rate_bps", 0))

    return Invoice(
        customer_id=account["customer_id"],
        subtotal_cents=subtotal_cents,
        discount_cents=discount_cents,
        taxable_cents=taxable_cents,
        tax_cents=tax_cents,
        total_cents=taxable_cents + tax_cents,
        line_items=tuple(line_items),
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
