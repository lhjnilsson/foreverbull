import pytest

from foreverbull.broker.credentials import set_cli_token


@pytest.fixture(autouse=True)
def _clear_cli_token():
    set_cli_token(None)
    yield
    set_cli_token(None)
