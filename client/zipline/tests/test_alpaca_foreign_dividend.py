"""One foreign Alpaca dividend, from a synthetic page through the stored count.

The page is
``pkg/finance/internal/suppliers/marketdata/testdata/alpaca_corporate_actions_tsm_foreign.json``.
It is synthetic, not a recorded Alpaca response. It holds one TSM cash
dividend: foreign, ex-date 2024-06-13, rate 0.539924, pay date 2024-07-11.
Bars are synthetic closes of 100 for the sessions 2024-06-10 through
2024-06-14, so the ex-date is inside the bars and is not the first one.
The strategy buys 10 shares on the first session.

The Go parser and this run are joined by hand. ``SQLIngester`` is called
with ``dividends=_dividend_frame(raw)``, the frame read from that page.
Nothing here calls the Go client. Ingest writes the dividend into the
bundle, the run books it, and the result the broker stores
(``foreign_dividends_gross``) is 1.
"""

import json
import logging
import os

from datetime import date
from pathlib import Path

import pandas as pd
import pytest

from foreverbull_zipline.data_bundles.dividends import DIVIDEND_COLUMNS
from foreverbull_zipline.data_bundles.dividends import read_foreign_dividends
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.engine import zero_benchmark
from foreverbull_zipline.returns import dividend_metrics
from protobuf import merge_from_binary
from zipline.api import order
from zipline.api import set_commission
from zipline.api import set_slippage
from zipline.api import symbol
from zipline.data import bundles
from zipline.data.data_portal import DataPortal
from zipline.extensions import load
from zipline.finance.blotter import Blotter
from zipline.finance.commission import PerShare
from zipline.finance.slippage import FixedSlippage
from zipline.finance.trading import SimulationParameters
from zipline.utils.calendar_utils import get_calendar

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb as engine_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from zipline import TradingAlgorithm


FIXTURE = (
    Path(__file__).resolve().parents[3]
    / "pkg/finance/internal/suppliers/marketdata/testdata/alpaca_corporate_actions_tsm_foreign.json"
)
EX_DATE = "2024-06-13"
FIRST_BAR = date(2024, 6, 10)
LAST_BAR = date(2024, 6, 14)
CLOSE = 100.0
CAPITAL = 10_000.0
SHARES = 10


class OHLCRows:
    def __init__(self, frames: dict[str, pd.DataFrame]):
        self.frames = frames

    def get_data(self, symbol_name, from_date, to_date):
        frame = self.frames[symbol_name]
        days = frame["time"].dt.date
        return frame[(days >= from_date) & (days <= to_date)].copy()


def _page() -> dict:
    return json.loads(FIXTURE.read_text(encoding="utf-8"))


def _one_foreign_dividend() -> dict:
    page = _page()
    dividends = page["corporate_actions"]["cash_dividends"]
    foreign = [row for row in dividends if row.get("foreign") is True]
    assert len(dividends) == 1
    assert len(foreign) == 1
    assert foreign[0]["symbol"] == "TSM"
    assert foreign[0]["ex_date"] == EX_DATE
    assert foreign[0]["rate"] == 0.539924
    return foreign[0]


def _bars() -> pd.DataFrame:
    calendar = get_calendar("XNYS")
    sessions = calendar.sessions_in_range(FIRST_BAR, LAST_BAR)
    stamps = [pd.Timestamp(day.year, day.month, day.day, 14, 30, tz="UTC") for day in sessions]
    count = len(stamps)
    return pd.DataFrame(
        {
            "time": stamps,
            "open": [CLOSE] * count,
            "high": [CLOSE] * count,
            "low": [CLOSE] * count,
            "close": [CLOSE] * count,
            "volume": [1_000] * count,
        }
    )


def _dividend_frame(raw: dict) -> pd.DataFrame:
    return pd.DataFrame(
        [
            {
                "symbol": raw["symbol"],
                "ex_date": pd.Timestamp(raw["ex_date"]),
                "pay_date": pd.Timestamp(raw["payable_date"]),
                "record_date": pd.Timestamp(raw["record_date"]),
                "declared_date": None,
                "amount": raw["rate"],
                "currency": raw.get("currency") or "USD",
                "is_foreign": bool(raw["foreign"]),
            }
        ],
        columns=pd.Index(DIVIDEND_COLUMNS),
    )


def _backtest() -> backtest_pb.Backtest:
    return backtest_pb.Backtest(
        name="tsm_foreign",
        provider="alpaca",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=common_pb.Date(year=2024, month=6, day=10),
                end_date=common_pb.Date(year=2024, month=6, day=14),
                assets=[backtest_pb.Backtest.Period.Asset(symbol="TSM", enabled=True)],
            )
        ],
    )


@pytest.fixture
def zipline_root(tmp_path, monkeypatch):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))


@pytest.mark.usefixtures("zipline_root")
def test_one_synthetic_tsm_foreign_dividend_is_stored_as_one(tmp_path):
    raw = _one_foreign_dividend()
    name = "tsm_foreign"
    ingester = SQLIngester(
        _backtest(),
        engine=OHLCRows({"TSM": _bars()}),
        aliases=pd.DataFrame(),
        events=pd.DataFrame(columns=pd.Index(["symbol", "kind", "on_date", "evidence"])),
        dividends=_dividend_frame(raw),
    )
    bundles.register(name, ingester, calendar_name="XNYS")
    try:
        bundles.ingest(name, os.environ, pd.Timestamp("2024-06-15", tz="UTC"), [], False)
        bundle = bundles.load(name, os.environ, None)
        foreign = read_foreign_dividends(bundle.adjustment_reader.conn)
        assert foreign is not None
        sid = int(bundle.asset_finder.lookup_symbol("TSM", as_of_date=None).sid)
        assert foreign == {(sid, pd.Timestamp(EX_DATE))}

        calendar = get_calendar("XNYS")
        sessions = calendar.sessions_in_range(FIRST_BAR, LAST_BAR)
        metrics_set, book = dividend_metrics(foreign)

        def initialize(context):
            set_slippage(us_equities=FixedSlippage(spread=0.0))
            set_commission(us_equities=PerShare(cost=0.0, min_trade_cost=0.0))
            context.asset = symbol("TSM")
            context.ordered = False

        def handle_data(context, data):
            if not context.ordered and data.can_trade(context.asset):
                order(context.asset, SHARES)
                context.ordered = True

        data_portal = DataPortal(
            bundle.asset_finder,
            trading_calendar=calendar,
            first_trading_day=bundle.equity_minute_bar_reader.first_trading_day,
            equity_minute_reader=bundle.equity_minute_bar_reader,
            equity_daily_reader=bundle.equity_daily_bar_reader,
            adjustment_reader=bundle.adjustment_reader,
        )
        logging.getLogger("zipline").setLevel(logging.ERROR)
        result = TradingAlgorithm(
            namespace={},
            data_portal=data_portal,
            trading_calendar=calendar,
            sim_params=SimulationParameters(
                start_session=sessions[0],
                end_session=sessions[-1],
                trading_calendar=calendar,
                data_frequency="daily",
                capital_base=CAPITAL,
            ),
            metrics_set=metrics_set,
            blotter=load(Blotter, "default"),
            benchmark_returns=zero_benchmark(sessions).total,
            benchmark_sid=None,
            initialize=initialize,
            handle_data=handle_data,
        ).run()

        assert book.foreign_booked == 1
        # 10 shares times the page's rate, credited at the ex-date open.
        booked = result.copy()
        booked.index = pd.DatetimeIndex([pd.Timestamp(day.date()) for day in booked.index])
        expected = SHARES * float(raw["rate"])
        ex = pd.Timestamp(EX_DATE)
        previous = pd.Timestamp("2024-06-12")
        assert float(booked.loc[ex, "dividends_earned"]) == pytest.approx(expected)
        cash_step = float(booked.loc[ex, "ending_cash"]) - float(booked.loc[previous, "ending_cash"])
        assert cash_step == pytest.approx(expected)
        engine = Engine(socket_file_path=str(tmp_path / "tsm.sock"))
        engine.result = result
        engine._benchmark = zero_benchmark(sessions)
        engine._capital_base = CAPITAL
        engine._has_benchmark = False
        engine._dividend_book = book
        response = engine_service_pb.GetResultResponse()
        merge_from_binary(response, engine._get_execution_result(engine_service_pb.GetResultRequest().to_binary()))
        assert response.foreign_dividends_gross == 1
    finally:
        bundles.unregister(name)
