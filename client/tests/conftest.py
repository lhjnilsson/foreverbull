import os

from functools import partial

import pandas as pd
import pytest

from zipline.api import order_target
from zipline.api import symbol
from zipline.data import bundles
from zipline.data.bundles import register
from zipline.errors import SymbolNotFound
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.run_algo import BenchmarkSpec
from zipline.utils.run_algo import _run

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2 as engine_service_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull_zipline import service
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester


@pytest.fixture(scope="session")
def backtest_entity():
    return backtest_pb2.Backtest(
        name="testing_backtest",
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=common_pb2.Date(year=2022, month=1, day=3),
                end_date=common_pb2.Date(year=2023, month=12, day=29),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AAPL",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AMZN",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="BAC",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="BRK-B",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="CMCSA",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="CSCO",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="DIS",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="GOOG",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="GOOGL",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="HD",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="INTC",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="JNJ",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="JPM",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="KO",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MA",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="META",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MRK",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MSFT",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="PEP",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="PG",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="TSLA",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="UNH",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="V",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="VZ",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="WMT",
                        enabled=True,
                    ),
                ],
            )
        ],
    )


@pytest.fixture(scope="session")
def foreverbull_bundle(backtest_entity: backtest_pb2.Backtest, fb_database):
    _, verify_or_populate = fb_database
    verify_or_populate(backtest_entity)

    def sanity_check(bundle):
        for period in backtest_entity.periods:
            for asset in period.assets:
                try:
                    stored_asset = bundle.asset_finder.lookup_symbol(asset.symbol, as_of_date=None)
                except SymbolNotFound:
                    raise LookupError(f"Asset {s} not found in bundle")

                backtest_start = pb_utils.from_proto_date_to_pandas_timestamp(period.start_date)
                if backtest_start < stored_asset.start_date:
                    raise ValueError("Start date is not correct")

                backtest_end = pb_utils.from_proto_date_to_pandas_timestamp(period.end_date)
                if backtest_end > stored_asset.end_date:
                    raise ValueError("End date is not correct")

    bundles.register("foreverbull", SQLIngester(backtest_entity), calendar_name="XNYS")
    try:
        sanity_check(bundles.load("foreverbull", os.environ, None))
    except (ValueError, LookupError):
        s = service.BacktestService()
        req = engine_service_pb2.IngestRequest(
            backtest=backtest_entity,
        )
        s.Ingest(req, None)


def baseline_performance_initialize(context):
    context.i = 0
    context.held_positions = []


def baseline_performance_handle_data(context, data, backtest: backtest_pb2.Backtest):
    context.i += 1
    if context.i < 30:
        return

    symbols = [asset.symbol for period in backtest.periods for asset in period.assets if asset.enabled]
    for s in symbols:
        short_mean = data.history(symbol(s), "close", bar_count=10, frequency="1d").mean()
        long_mean = data.history(symbol(s), "close", bar_count=30, frequency="1d").mean()
        if short_mean > long_mean and s not in context.held_positions:
            order_target(symbol(s), 10)
            context.held_positions.append(s)
        elif short_mean < long_mean and s in context.held_positions:
            order_target(symbol(s), 0)
            context.held_positions.remove(s)


@pytest.fixture(scope="session")
def baseline_performance(foreverbull_bundle, backtest_entity: backtest_pb2.Backtest):
    register("foreverbull", SQLIngester(backtest_entity), calendar_name="XNYS")
    benchmark_spec = BenchmarkSpec.from_cli_params(
        no_benchmark=True,
        benchmark_sid=None,
        benchmark_symbol=None,
        benchmark_file=None,
    )
    if os.path.exists("baseline_performance.pickle"):
        os.remove("baseline_performance.pickle")

    _run(
        initialize=baseline_performance_initialize,
        handle_data=partial(baseline_performance_handle_data, backtest=backtest_entity),
        before_trading_start=None,
        analyze=None,
        algofile=None,
        algotext=None,
        defines=None,
        data_frequency="daily",
        capital_base=100000,
        bundle="foreverbull",
        bundle_timestamp=pd.Timestamp.utcnow(),
        start=pb_utils.from_proto_date_to_pandas_timestamp(backtest_entity.periods[0].start_date),
        end=pb_utils.from_proto_date_to_pandas_timestamp(backtest_entity.periods[-1].end_date),
        output="baseline_performance.pickle",
        trading_calendar=get_calendar("XNYS"),
        print_algo=False,
        metrics_set="default",
        local_namespace=None,
        environ=os.environ,
        blotter="default",
        benchmark_spec=benchmark_spec,
        custom_loader=None,
    )

    return pd.read_pickle("baseline_performance.pickle").reset_index(drop=True)
