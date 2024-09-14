import os
import tempfile
from datetime import datetime
from threading import Thread

import pandas
import pynng
import pytest
from foreverbull import entity
from foreverbull.algorithm import Algorithm
from foreverbull.models import Algorithm, Asset, Assets
from foreverbull.pb.service import service_pb2, worker_pb2


class TestAsset:
    def test_asset_getattr_setattr(self, fb_database, namespace_server):
        database, _ = fb_database
        with database.connect() as conn:
            asset = Asset(datetime.now(), conn, "AAPL")
            assert asset is not None
            asset.set_metric("rsi", 56.4)

            assert "rsi" in namespace_server
            assert namespace_server["rsi"] == {"AAPL": 56.4}

            namespace_server["pe"] = {"AAPL": 12.3}
            assert asset.get_metric("pe") == 12.3

    def test_assets(self, fb_database, backtest_entity):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        with database.connect() as conn:
            assets = Assets(datetime.now(), conn, backtest_entity.symbols)
            for asset in assets:
                assert asset is not None
                assert asset.symbol is not None
                stock_data = asset.stock_data
                assert stock_data is not None
                assert isinstance(stock_data, pandas.DataFrame)
                assert len(stock_data) > 0
                assert "open" in stock_data.columns
                assert "high" in stock_data.columns
                assert "low" in stock_data.columns
                assert "close" in stock_data.columns
                assert "volume" in stock_data.columns


class TestAssets:
    def test_assets_getattr_setattr(self, fb_database, namespace_server):
        database, _ = fb_database
        with database.connect() as conn:
            assets = Assets(datetime.now(), conn, [])
            assert assets is not None
            assets.set_metrics("holdings", {"AAPL": True, "MSFT": False})

            assert "holdings" in namespace_server
            assert namespace_server["holdings"] == {"AAPL": True, "MSFT": False}

            namespace_server["pe"] = {"AAPL": 12.3, "MSFT": 23.4}
            assert assets.get_metrics("pe") == {"AAPL": 12.3, "MSFT": 23.4}


class TestPortfolio:
    pass


class TestNonParallel:
    @pytest.fixture
    def non_parallel_algo(self):
        example = b"""
from foreverbull import Algorithm, Function, Assets, Portfolio, Order

def handle_data(low: int, high: int, assets: Assets, portfolio: Portfolio) -> list[Order]:
    pass

Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
    """
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(example)
            f.flush()
            yield Algorithm.from_file_path(f.name)

    def test_non_parallel(self, non_parallel_algo: Algorithm):
        assert non_parallel_algo.get_entity() == entity.service.Service.Algorithm(
            file_path=non_parallel_algo._file_path,
            functions=[
                entity.service.Service.Algorithm.Function(
                    name="handle_data",
                    parameters=[
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="low",
                            type="int",
                        ),
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="high",
                            type="int",
                        ),
                    ],
                    parallel_execution=False,
                    run_first=False,
                    run_last=False,
                ),
            ],
            namespaces=[],
        )
        non_parallel_algo.configure("handle_data", "low", "5")
        non_parallel_algo.configure("handle_data", "high", "10")

    @pytest.fixture
    def parallel_algo(self):
        example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass

Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(example)
            f.flush()
            yield Algorithm.from_file_path(f.name)

    def test_parallel_algo(self, parallel_algo: Algorithm):
        assert parallel_algo.get_entity() == entity.service.Service.Algorithm(
            file_path=parallel_algo._file_path,
            functions=[
                entity.service.Service.Algorithm.Function(
                    name="handle_data",
                    parameters=[
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="low",
                            default="5",
                            type="int",
                        ),
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="high",
                            default="10",
                            type="int",
                        ),
                    ],
                    parallel_execution=True,
                    run_first=False,
                    run_last=False,
                ),
            ],
            namespaces=[],
        )
        parallel_algo.configure("handle_data", "low", "5")
        parallel_algo.configure("handle_data", "high", "10")

    @pytest.fixture
    def algo_with_namespace(self):
        example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass

Algorithm(
    functions=[
        Function(callable=handle_data)
    ],
    namespaces=["qualified_symbols", "rsi"]
)
"""

        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(example)
            f.flush()
            yield Algorithm.from_file_path(f.name)

    def test_entity(self, algo_with_namespace: Algorithm):
        assert algo_with_namespace.get_entity() == entity.service.Service.Algorithm(
            file_path=algo_with_namespace._file_path,
            functions=[
                entity.service.Service.Algorithm.Function(
                    name="handle_data",
                    parameters=[
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="low",
                            default="5",
                            type="int",
                        ),
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="high",
                            default="10",
                            type="int",
                        ),
                    ],
                    parallel_execution=True,
                    run_first=False,
                    run_last=False,
                ),
            ],
            namespaces=["qualified_symbols", "rsi"],
        )
        algo_with_namespace.configure("handle_data", "low", "5")
        algo_with_namespace.configure("handle_data", "high", "10")


class TestMultiStepWithNamespace:
    @pytest.fixture
    def multistep_algo_with_namespace(self):
        example = b"""
from foreverbull import Algorithm, Function, Asset, Assets, Portfolio, Order


def measure_assets(asset: Asset, low: int = 5, high: int = 10) -> None:
    pass

def create_orders(assets: Assets, portfolio: Portfolio) -> list[Order]:
    pass

def filter_assets(assets: Assets) -> None:
    pass

Algorithm(
    functions=[
        Function(callable=measure_assets),
        Function(callable=create_orders, run_last=True),
        Function(callable=filter_assets, run_first=True),
    ],
    namespaces=["qualified_symbols", "asset_metrics"]
)
"""

        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(example)
            f.flush()
            yield Algorithm.from_file_path(f.name)

    def test_entity(self, multistep_algo_with_namespace: Algorithm):
        assert multistep_algo_with_namespace.get_entity() == entity.service.Service.Algorithm(
            file_path=multistep_algo_with_namespace._file_path,
            functions=[
                entity.service.Service.Algorithm.Function(
                    name="measure_assets",
                    parameters=[
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="low",
                            default="5",
                            type="int",
                        ),
                        entity.service.Service.Algorithm.Function.Parameter(
                            key="high",
                            default="10",
                            type="int",
                        ),
                    ],
                    parallel_execution=True,
                    run_first=False,
                    run_last=False,
                ),
                entity.service.Service.Algorithm.Function(
                    name="create_orders",
                    parameters=[],
                    parallel_execution=False,
                    run_first=False,
                    run_last=True,
                ),
                entity.service.Service.Algorithm.Function(
                    name="filter_assets",
                    parameters=[],
                    parallel_execution=False,
                    run_first=True,
                    run_last=False,
                ),
            ],
            namespaces=["qualified_symbols", "asset_metrics"],
        )
        multistep_algo_with_namespace.configure("measure_assets", "low", "5")
        multistep_algo_with_namespace.configure("measure_assets", "high", "10")
