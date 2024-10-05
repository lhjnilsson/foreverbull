import os
from functools import partial

import pandas as pd
import pytest
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import (
    backtest_pb2,
    engine_service_pb2,
    execution_pb2,
    ingestion_pb2,
)
from foreverbull_zipline import engine
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from zipline.api import order_target, symbol
from zipline.data import bundles
from zipline.data.bundles import register
from zipline.errors import SymbolNotFound
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.run_algo import BenchmarkSpec, _run


@pytest.fixture(scope="session")
def execution() -> execution_pb2.Execution:
    return execution_pb2.Execution(
        start_date=common_pb2.Date(year=2022, month=1, day=3),
        end_date=common_pb2.Date(year=2023, month=12, day=29),
        benchmark=None,
        symbols=[
            "AAPL",
            "AMZN",
            "BAC",
            "BRK-B",
            "CMCSA",
            "CSCO",
            "DIS",
            "GOOG",
            "GOOGL",
            "HD",
            "INTC",
            "JNJ",
            "JPM",
            "KO",
            "MA",
            "META",
            "MRK",
            "MSFT",
            "PEP",
            "PG",
            "TSLA",
            "UNH",
            "V",
            "VZ",
            "WMT",
        ],
    )


@pytest.fixture(scope="session")
def foreverbull_bundle(execution: execution_pb2.Execution, fb_database):
    _, verify_or_populate = fb_database

    backtest_entity = backtest_pb2.Backtest(
        name="test_backtest",
        start_date=execution.start_date,
        end_date=execution.end_date,
        symbols=execution.symbols,
    )
    verify_or_populate(execution)

    def sanity_check(bundle):
        for s in backtest_entity.symbols:
            try:
                stored_asset = bundle.asset_finder.lookup_symbol(s, as_of_date=None)
            except SymbolNotFound:
                raise LookupError(f"Asset {s} not found in bundle")

            backtest_start = (pb_utils.from_proto_date_to_pandas_timestamp(execution.start_date),)
            if backtest_start < stored_asset.start_date:
                print("Start date is not correct", backtest_start, stored_asset.start_date)
                raise ValueError("Start date is not correct")

            backtest_end = pb_utils.from_proto_date_to_pandas_timestamp(execution.end_date)
            if backtest_end > stored_asset.end_date:
                print("End date is not correct", backtest_end, stored_asset.end_date)
                raise ValueError("End date is not correct")

    bundles.register("foreverbull", SQLIngester(), calendar_name="XNYS")
    try:
        sanity_check(bundles.load("foreverbull", os.environ, None))
    except (ValueError, LookupError):
        e = engine.EngineProcess()
        req = engine_service_pb2.IngestRequest(
            ingestion=ingestion_pb2.Ingestion(
                start_date=backtest_entity.start_date,
                end_date=backtest_entity.end_date,
                symbols=backtest_entity.symbols,
            )
        )
        e._ingest(req.SerializeToString())


def baseline_performance_initialize(context):
    context.i = 0
    context.held_positions = []


def baseline_performance_handle_data(context, data, execution: execution_pb2.Execution):
    context.i += 1
    if context.i < 30:
        return

    for s in execution.symbols:
        short_mean = data.history(symbol(s), "close", bar_count=10, frequency="1d").mean()
        long_mean = data.history(symbol(s), "close", bar_count=30, frequency="1d").mean()
        if short_mean > long_mean and s not in context.held_positions:
            order_target(symbol(s), 10)
            context.held_positions.append(s)
        elif short_mean < long_mean and s in context.held_positions:
            order_target(symbol(s), 0)
            context.held_positions.remove(s)


@pytest.fixture(scope="session")
def baseline_performance(foreverbull_bundle, execution: execution_pb2.Execution):
    register("foreverbull", SQLIngester(), calendar_name="XNYS")
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
        handle_data=partial(baseline_performance_handle_data, execution=execution),
        before_trading_start=None,
        analyze=None,
        algofile=None,
        algotext=None,
        defines=None,
        data_frequency="daily",
        capital_base=100000,
        bundle="foreverbull",
        bundle_timestamp=pd.Timestamp.utcnow(),
        start=pb_utils.from_proto_date_to_pandas_timestamp(execution.start_date),
        end=pb_utils.from_proto_date_to_pandas_timestamp(execution.end_date),
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
