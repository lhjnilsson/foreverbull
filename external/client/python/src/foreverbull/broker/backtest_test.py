from datetime import datetime
from unittest.mock import patch

import pytest

from foreverbull.broker import backtest
from foreverbull.entity.backtest import Backtest, Execution, Session


@pytest.mark.parametrize(
    "return_value, expected_model",
    [
        (
            [],
            [],
        ),
    ],
)
def test_backtest_list(
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.list() == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            Backtest(
                name="test_service",
                start=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                end=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                symbols=[
                    "AAPL",
                    "MSFT",
                ],
            ),
            {
                "name": "test_service",
                "start": "2021-01-01T00:00:00",
                "end": "2021-01-01T00:00:00",
                "symbols": [
                    "AAPL",
                    "MSFT",
                ],
            },
            Backtest(
                name="test_service",
                start=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                end=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                symbols=[
                    "AAPL",
                    "MSFT",
                ],
            ),
        ),
    ],
)
def test_backtest_create(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.create(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_service",
            {
                "name": "test_service",
                "start": "2021-01-01T00:00:00",
                "end": "2021-01-01T00:00:00",
                "symbols": [
                    "AAPL",
                    "MSFT",
                ],
            },
            Backtest(
                name="test_service",
                start=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                end=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                symbols=[
                    "AAPL",
                    "MSFT",
                ],
            ),
        ),
    ],
)
def test_backtest_get(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.get(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_service",
            [],
            [],
        ),
    ],
)
def test_backtest_list_sessions(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.list_sessions(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_service",
            {
                "id": "123",
                "backtest": "test_backtest",
                "manual": False,
                "executions": 1,
                "statuses": [],
                "socket": None,
            },
            Session(
                id="123",
                backtest="test_backtest",
                manual=False,
                executions=1,
                statuses=[],
                socket=None,
            ),
        ),
    ],
)
def test_backtest_run(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.run(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_service",
            {
                "id": "123",
                "backtest": "test_backtest",
                "manual": False,
                "executions": 1,
                "statuses": [],
                "socket": None,
            },
            Session(
                id="123",
                backtest="test_backtest",
                manual=False,
                executions=1,
                statuses=[],
                socket=None,
            ),
        ),
    ],
)
def test_backtest_get_session(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.get_session(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_service",
            [],
            [],
        ),
    ],
)
def test_backtest_list_executions(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.list_executions(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_service",
            {
                "id": "test_service",
                "name": "test_service",
                "start": "2021-01-01T00:00:00",
                "end": "2021-01-01T00:00:00",
                "symbols": [
                    "AAPL",
                    "MSFT",
                ],
            },
            Execution(
                id="test_service",
                name="test_service",
                start=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                end=datetime.strptime(
                    "2021-01-01",
                    "%Y-%m-%d",
                ),
                symbols=[
                    "AAPL",
                    "MSFT",
                ],
            ),
        ),
    ],
)
def test_backtest_get_execution(
    argument,
    return_value,
    expected_model,
):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert backtest.get_execution(argument) == expected_model
        mock_send.assert_called_once()
