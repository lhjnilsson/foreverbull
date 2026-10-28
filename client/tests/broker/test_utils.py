"""Redaction never shows a database password in free text."""

import pytest

from foreverbull.broker.utils import redact_text


SECRET = "n0t-the-default-Pa55"


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        (f"at postgresql://u:{SECRET}@h/db.", "at postgresql://u:***@h/db."),
        (f"at postgresql+psycopg://u:{SECRET}@h/db", "at postgresql+psycopg://u:***@h/db"),
        ("postgresql://u@h/db has no password", "postgresql://u@h/db has no password"),
        (f"password {SECRET} quoted bare", "password *** quoted bare"),
    ],
)
def test_redact_text(text, expected):
    assert redact_text(text, SECRET) == expected
