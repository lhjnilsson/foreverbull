from contextlib import contextmanager
from unittest.mock import MagicMock
from unittest.mock import patch

import grpc
import pytest

from foreverbull_cli.bull.cli import cli
from foreverbull_cli.bull.trading import PositionAndOrder
from typer.testing import CliRunner

from foreverbull.foreverbull import TradingSession
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb as trading_service_pb
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.finance.v1 import trading_pb


pytestmark = pytest.mark.usefixtures("fb_env")


class _LookupError(grpc.RpcError):
    def __init__(self, details: str, code: grpc.StatusCode):
        self._details = details
        self._code = code

    def details(self):
        return self._details

    def code(self):
        return self._code


def _sample_backtest(name="test_name") -> backtest_pb.Backtest:
    return backtest_pb.Backtest(
        name=name,
        periods=[
            backtest_pb.Backtest.Period(
                assets=[
                    backtest_pb.Backtest.Period.Asset(symbol="A", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="B", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="C", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="D", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="E", enabled=False),
                ]
            )
        ],
    )


def _sample_orders() -> list[marketdata_pb.Order]:
    return [
        marketdata_pb.Order(symbol="A", amount=10),
        marketdata_pb.Order(symbol="B", amount=-10),
        marketdata_pb.Order(symbol="E", amount=10),
        marketdata_pb.Order(symbol="F", amount=10),
    ]


def _sample_portfolio() -> marketdata_pb.Portfolio:
    return marketdata_pb.Portfolio(
        positions=[
            marketdata_pb.Position(symbol="A", amount=10),
            marketdata_pb.Position(symbol="B", amount=10),
            marketdata_pb.Position(symbol="C", amount=10),
            marketdata_pb.Position(symbol="D", amount=10),
        ]
    )


class TestPositionAndOrder:
    @pytest.mark.parametrize(
        "positions,orders,expected_length",
        [
            ([], [], 0),
            (
                [
                    marketdata_pb.Position(symbol="A", amount=10),
                    marketdata_pb.Position(symbol="B", amount=10),
                    marketdata_pb.Position(symbol="C", amount=10),
                    marketdata_pb.Position(symbol="D", amount=10),
                ],
                [
                    marketdata_pb.Order(symbol="C", amount=10),
                    marketdata_pb.Order(symbol="E", amount=10),
                ],
                5,
            ),
        ],
    )
    def test_parse(self, positions, orders, expected_length):
        items = PositionAndOrder.parse(positions, orders)
        assert len(items) == expected_length

    @pytest.mark.parametrize(
        "position,order,expected_intent",
        [
            (
                None,
                marketdata_pb.Order(symbol="A", amount=10),
                marketdata_pb.PositionIntent.BUY_TO_OPEN,
            ),
            (
                None,
                marketdata_pb.Order(symbol="A", amount=-10),
                marketdata_pb.PositionIntent.SELL_TO_OPEN,
            ),
            (
                marketdata_pb.Position(symbol="A", amount=10),
                marketdata_pb.Order(symbol="A", amount=-10),
                marketdata_pb.PositionIntent.SELL_TO_CLOSE,
            ),
            (
                marketdata_pb.Position(symbol="A", amount=-10),
                marketdata_pb.Order(symbol="A", amount=10),
                marketdata_pb.PositionIntent.BUY_TO_CLOSE,
            ),
        ],
    )
    def test_with_defaults_sets_position_intent(self, position, order, expected_intent):
        item = PositionAndOrder.with_defaults(position, order)
        assert item.order is not None
        assert item.order.intent == expected_intent
        assert item.order.type == marketdata_pb.OrderType.MARKET
        assert item.order.time_in_force == marketdata_pb.TimeInForce.DAY

    def test_display_and_rejects_both_none(self):
        with pytest.raises(ValueError, match="Both position and order are None"):
            PositionAndOrder(None, None)

        held = PositionAndOrder(marketdata_pb.Position(symbol="A", amount=10, cost_basis=12.5), None)
        assert held.symbol == "A"
        assert held.amount == "10"
        assert held.cost_basis == "12.5"
        assert held.order_type == "-"
        assert held.time_in_force == "-"
        assert held.position_intent == "-"

        opening = PositionAndOrder.with_defaults(None, marketdata_pb.Order(symbol="B", amount=5))
        assert opening.symbol == "B"
        assert opening.amount == "0 -> 5"
        assert opening.cost_basis == "-"
        assert opening.order_type == str(marketdata_pb.OrderType.MARKET)


@contextmanager
def _mock_live_trading():
    mock_session = MagicMock(spec=TradingSession)
    place_orders = MagicMock()
    mock_session.run_trading.return_value = (_sample_orders(), place_orders)
    with (
        patch("foreverbull_cli.bull.trading.Foreverbull") as mock_foreverbull,
        patch("foreverbull.models.Algorithm.from_file_path") as from_file_path,
        patch("foreverbull.broker.finance.get_portfolio") as get_portfolio,
        patch("foreverbull.broker.backtest.get") as get_backtest,
        patch("foreverbull.broker.finance.create_session") as create_session,
    ):
        from_file_path.return_value = True
        get_backtest.return_value = _sample_backtest()
        create_session.return_value = iter(
            [trading_service_pb.CreateSessionResponse(session=trading_pb.Session(port=7878))]
        )
        get_portfolio.return_value = trading_service_pb.GetPortfolioResponse(portfolio=_sample_portfolio())
        mock_foreverbull.return_value.trading_session.return_value.__enter__.return_value = mock_session
        yield {
            "place_orders": place_orders,
            "create_session": create_session,
            "get_portfolio": get_portfolio,
            "session": mock_session,
        }


def test_trading_run_yes(runner):
    with _mock_live_trading() as mocks:
        result = runner(cli, ["trading", "run", "test_name", "demo.py", "--yes"])
        assert result.exit_code == 0
        assert "Positions and Orders" in result.stdout
        mocks["place_orders"].assert_called_once()
        mocks["create_session"].assert_called_once()


def test_trading_run_dry(runner):
    with (
        patch("foreverbull.broker.backtest.get") as get_backtest,
        patch("foreverbull.broker.finance.create_session") as create_session,
        patch("foreverbull.broker.finance.get_portfolio") as get_portfolio,
    ):
        get_backtest.return_value = _sample_backtest()
        result = runner(cli, ["trading", "run", "test_name", "demo.py", "--dry"])
        assert result.exit_code == 0
        assert "Dry Run" in result.stdout
        assert "demo.py" in result.stdout
        assert "test_name" in result.stdout
        assert "A, B, C, D" in result.stdout
        create_session.assert_not_called()
        get_portfolio.assert_not_called()


def test_trading_run_dry_overrides_yes(runner):
    with (
        patch("foreverbull.broker.backtest.get") as get_backtest,
        patch("foreverbull.broker.finance.create_session") as create_session,
        patch("foreverbull.broker.finance.get_portfolio") as get_portfolio,
    ):
        get_backtest.return_value = _sample_backtest()
        result = runner(cli, ["trading", "run", "test_name", "demo.py", "--dry", "--yes"])
        assert result.exit_code == 0
        assert "Dry Run" in result.stdout
        create_session.assert_not_called()
        get_portfolio.assert_not_called()


def test_trading_run_requires_confirmation(runner):
    with _mock_live_trading() as mocks:
        result = runner(cli, ["trading", "run", "test_name", "demo.py"], input="n\n")
        assert result.exit_code == 0
        mocks["place_orders"].assert_not_called()


def test_trading_run_confirm_places(runner):
    with _mock_live_trading() as mocks:
        result = runner(cli, ["trading", "run", "test_name", "demo.py"], input="y\n")
        assert result.exit_code == 0
        mocks["place_orders"].assert_called_once()


# One ✖ line and exit 1 that says why there is nothing to trade and how to fix it.
def test_trading_run_no_periods_fails_with_one_line(monkeypatch):
    monkeypatch.setenv("TTY_COMPATIBLE", "0")
    with (
        patch("foreverbull.broker.backtest.get") as get_backtest,
        patch("foreverbull.broker.finance.create_session") as create_session,
        patch("foreverbull.broker.finance.get_portfolio") as get_portfolio,
    ):
        get_backtest.return_value = backtest_pb.Backtest(name="empty")
        result = CliRunner().invoke(cli, ["trading", "run", "empty", "demo.py", "--yes"])
    assert result.exit_code == 1
    assert isinstance(result.exception, SystemExit)
    assert result.stdout.count("✖") == 1
    assert (
        "✖ Backtest 'empty' has no data, so there are no symbols to trade. Pick another backtest, "
        "or delete it with `fbull backtest delete empty` and create it again with `fbull backtest create`."
    ) in " ".join(result.stdout.split())
    assert "has no periods" not in result.stdout
    assert "Traceback" not in result.stdout
    create_session.assert_not_called()
    get_portfolio.assert_not_called()


def test_trading_run_unknown_backtest_prints_one_plain_line(monkeypatch):
    monkeypatch.setenv("TTY_COMPATIBLE", "0")
    missing = _LookupError("backtest typo not found", grpc.StatusCode.NOT_FOUND)
    with (
        patch("foreverbull.broker.backtest.get", side_effect=missing),
        patch("foreverbull.broker.finance.create_session") as create_session,
        patch("foreverbull.broker.finance.get_portfolio") as get_portfolio,
    ):
        result = CliRunner().invoke(cli, ["trading", "run", "typo", "demo.py", "--yes"])

    assert result.exit_code == 1
    assert result.output == (
        "✖ There's no backtest named 'typo'. Run `fbull backtest list` to see your "
        "backtests, or create it with `fbull backtest create`.\n"
    )
    create_session.assert_not_called()
    get_portfolio.assert_not_called()


def test_trading_run_other_lookup_errors_are_not_reported_as_missing():
    broken = _LookupError("Socket closed", grpc.StatusCode.UNAVAILABLE)
    with patch("foreverbull.broker.backtest.get", side_effect=broken):
        result = CliRunner().invoke(cli, ["trading", "run", "demo", "demo.py", "--yes"])

    assert result.exit_code != 0
    assert result.exception is broken
    assert "no backtest named" not in result.output
