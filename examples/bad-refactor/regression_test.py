import unittest

from app import EXAMPLE_ACCOUNT, calculate_invoice


class InvoiceRegressionTests(unittest.TestCase):
    def test_suspended_users_are_not_billed(self):
        invoice = calculate_invoice(EXAMPLE_ACCOUNT)
        line_items = dict(invoice.line_items)

        self.assertEqual(line_items["extra seats"], 28_000)
        self.assertEqual(invoice.subtotal_cents, 120_600)
        self.assertEqual(invoice.taxable_cents, 110_000)
        self.assertEqual(invoice.total_cents, 119_625)


if __name__ == "__main__":
    unittest.main()
