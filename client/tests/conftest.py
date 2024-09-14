import os
import time
from datetime import datetime, timezone
from functools import partial
from multiprocessing import get_start_method, set_start_method
from typing import Generator

import pandas as pd
import pynng
import pytest
from foreverbull import Algorithm, entity
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, engine_pb2
from foreverbull.pb.finance import finance_pb2
from foreverbull.pb.service import service_pb2, worker_pb2
from foreverbull_zipline import engine, grpc_servicer
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from zipline.api import order_target, symbol
from zipline.data import bundles
from zipline.data.bundles import register
from zipline.errors import SymbolNotFound
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.run_algo import BenchmarkSpec, _run


@pytest.fixture(scope="session")
def execution() -> entity.backtest.Execution:
    return entity.backtest.Execution(
        start=datetime(2022, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 12, 29, tzinfo=timezone.utc),
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
def foreverbull_bundle(execution: entity.backtest.Execution, fb_database):
    backtest_entity = entity.backtest.Backtest(
        name="test_backtest",
        start=execution.start,
        end=execution.end,
        symbols=execution.symbols,
    )

    def sanity_check(bundle):
        for s in backtest_entity.symbols:
            try:
                stored_asset = bundle.asset_finder.lookup_symbol(s, as_of_date=None)
            except SymbolNotFound:
                raise LookupError(f"Asset {s} not found in bundle")
            backtest_start = pd.Timestamp(backtest_entity.start).normalize().tz_localize(None)
            if backtest_start < stored_asset.start_date:
                print("Start date is not correct", backtest_start, stored_asset.start_date)
                raise ValueError("Start date is not correct")

            backtest_end = pd.Timestamp(backtest_entity.end).normalize().tz_localize(None)
            if backtest_end > stored_asset.end_date:
                print("End date is not correct", backtest_end, stored_asset.end_date)
                raise ValueError("End date is not correct")

    bundles.register("foreverbull", SQLIngester(), calendar_name="XNYS")
    try:
        print("Loading bundle")
        sanity_check(bundles.load("foreverbull", os.environ, None))
    except (ValueError, LookupError) as exc:
        print("Creating bundle", exc)
        e = engine.EngineProcess()
        req = engine_pb2.IngestRequest(
            ingestion=backtest_pb2.Ingestion(
                start_date=pb_utils.to_proto_timestamp(backtest_entity.start),
                end_date=pb_utils.to_proto_timestamp(backtest_entity.end),
                symbols=backtest_entity.symbols,
            )
        )
        e._ingest(req.SerializeToString())


def baseline_performance_initialize(context):
    context.i = 0
    context.held_positions = []


def baseline_performance_handle_data(context, data, execution: entity.backtest.Execution):
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
def baseline_performance(foreverbull_bundle, execution):
    register("foreverbull", SQLIngester(), calendar_name="XNYS")
    benchmark_spec = BenchmarkSpec.from_cli_params(
        no_benchmark=True,
        benchmark_sid=None,
        benchmark_symbol=None,
        benchmark_file=None,
    )
    if os.path.exists("baseline_performance.pickle"):
        os.remove("baseline_performance.pickle")

    trading_calendar = get_calendar("XNYS")
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
        start=pd.Timestamp(execution.start).normalize().tz_localize(None),
        end=pd.Timestamp(execution.end).normalize().tz_localize(None),
        output="baseline_performance.pickle",
        trading_calendar=trading_calendar,
        print_algo=False,
        metrics_set="default",
        local_namespace=None,
        environ=os.environ,
        blotter="default",
        benchmark_spec=benchmark_spec,
        custom_loader=None,
    )

    return pd.read_pickle("baseline_performance.pickle").reset_index(drop=True)
