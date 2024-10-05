from datetime import datetime, timezone
from multiprocessing import get_start_method, set_start_method

import pytest
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import backtest_pb2, execution_pb2
from foreverbull.pb.pb_utils import to_proto_timestamp


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="session")
def backtest_entity():
    return backtest_pb2.Backtest(
        name="testing_backtest",
        start_date=common_pb2.Date(year=2022, month=1, day=3),
        end_date=common_pb2.Date(year=2023, month=12, day=29),
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


@pytest.fixture(scope="function")
def execution():
    return execution_pb2.Execution(
        id="test",
        start_date=common_pb2.Date(year=2022, month=1, day=3),
        end_date=common_pb2.Date(year=2023, month=12, day=29),
        symbols=["AAPL", "MSFT", "TSLA"],
        benchmark="AAPL",
    )
