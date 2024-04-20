from unittest.mock import patch

import pytest

from foreverbull.broker import finance


@pytest.mark.parametrize(
    "return_value, expected_model",
    [
        (
            [],
            [],
        ),
    ],
)
def test_finance_get_assets(
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert finance.get_assets() == expected_model
        mock_send.assert_called_once()
