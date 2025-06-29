from unittest.mock import MagicMock
from unittest.mock import patch

import pytest

from foreverbull.foreverbull import Foreverbull
from foreverbull.foreverbull import TradingSession
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2 as trading_service_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.finance.v1 import trading_pb2
from foreverbull_cli.bull.trading import PositionAndOrder
from foreverbull_cli.bull.trading import trading


class TestPositionAndOrder:
    @pytest.mark.parametrize(
        "positions,orders,expected_length",
        [
            ([], [], 0),
            (
                [
                    marketdata_pb2.Position(symbol="A", amount=10),
                    marketdata_pb2.Position(symbol="B", amount=10),
                    marketdata_pb2.Position(symbol="C", amount=10),
                    marketdata_pb2.Position(symbol="D", amount=10),
                ],
                [
                    marketdata_pb2.Order(symbol="C", amount=10),
                    marketdata_pb2.Order(symbol="E", amount=10),
                ],
                5,
            ),
        ],
    )
    def test_parse(self, positions, orders, expected_length):
        items = PositionAndOrder.parse(positions, orders)
        assert len(items) == expected_length


def test_session_run(runner):
    mock_trading_session = MagicMock(spec=TradingSession)

    mock_foreverbull = MagicMock(spec=Foreverbull)
    mock_foreverbull.return_value.__enter__.return_value = mock_trading_session

    with (
        patch("foreverbull.foreverbull.Foreverbull.trading_session", new_callable=lambda: mock_foreverbull),
        patch("foreverbull.broker.finance.get_portfolio") as get_portfolio,
        patch("foreverbull.broker.backtest.get") as get_backtest,
        patch("foreverbull.broker.finance.create_session") as create_session,
        patch("foreverbull.models.Algorithm.from_file_path") as from_file_path,
    ):
        from_file_path.return_value = True
        get_backtest.return_value = backtest_pb2.Backtest(symbols=["A", "B", "C", "D", "E", "F"])
        create_session.return_value = iter(
            [trading_service_pb2.CreateSessionResponse(session=trading_pb2.Session(port=7878))]
        )
        portfolio = marketdata_pb2.Portfolio(
            positions=[
                marketdata_pb2.Position(symbol="A", amount=10),
                marketdata_pb2.Position(symbol="B", amount=10),
                marketdata_pb2.Position(symbol="C", amount=10),
                marketdata_pb2.Position(symbol="D", amount=10),
            ]
        )
        get_portfolio.return_value = trading_service_pb2.GetPortfolioResponse(portfolio=portfolio)
        mock_trading_session.run_trading.return_value = tuple(
            [
                [
                    marketdata_pb2.Order(symbol="A", amount=10),
                    marketdata_pb2.Order(symbol="B", amount=-10),
                    marketdata_pb2.Order(symbol="E", amount=10),
                    marketdata_pb2.Order(symbol="F", amount=10),
                ],
                None,
            ]
        )

        result = runner(trading, ["run", "test"])
        assert result.exit_code == 0
