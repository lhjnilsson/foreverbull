from multiprocessing import get_start_method
from multiprocessing import set_start_method

import pytest

from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2


@pytest.fixture(autouse=True, scope="session")
def spawn_process():
    print("IS USED")
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="session")
def backtest_entity():
    return backtest_pb2.Backtest(
        name="testing_backtest",
        benchmark="OEF",
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=common_pb2.Date(year=2022, month=1, day=1),
                end_date=common_pb2.Date(year=2022, month=12, day=31),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AAPL",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MSFT",
                        enabled=True,
                    ),
                ],
            ),
            backtest_pb2.Backtest.Period(
                start_date=common_pb2.Date(year=2023, month=1, day=1),
                end_date=common_pb2.Date(year=2023, month=12, day=31),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AAPL",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AMZN",
                        enabled=True,
                    ),
                ],
            ),
        ],
    )


@pytest.fixture(scope="function")
def execution(backtest_entity):
    return execution_pb2.Execution(
        id="test",
        backtest=backtest_entity.name,
        start_date=common_pb2.Date(year=2022, month=1, day=1),
        end_date=common_pb2.Date(year=2023, month=12, day=31),
    )
