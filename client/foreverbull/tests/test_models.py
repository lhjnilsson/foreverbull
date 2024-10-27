import tempfile

from datetime import datetime
from unittest import mock

import pandas
import pytest

from foreverbull.models import Algorithm
from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.pb.foreverbull.service import worker_pb2


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


class TestDefinitions:
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
        assert non_parallel_algo._file_path is not None
        assert non_parallel_algo._functions is not None
        assert len(non_parallel_algo._functions) == 1
        assert "handle_data" in non_parallel_algo._functions
        assert non_parallel_algo._functions["handle_data"] == {
            "callable": mock.ANY,
            "asset_key": "assets",
            "portfolio_key": "portfolio",
            "definition": worker_pb2.Algorithm.Function(
                name="handle_data",
                parameters=[
                    worker_pb2.Algorithm.FunctionParameter(
                        key="low",
                        valueType="int",
                    ),
                    worker_pb2.Algorithm.FunctionParameter(
                        key="high",
                        valueType="int",
                    ),
                ],
                parallelExecution=False,
                runFirst=False,
                runLast=False,
            ),
        }
        assert non_parallel_algo._namespaces == []
        non_parallel_algo.configure("handle_data", "low", "5")
        non_parallel_algo.configure("handle_data", "high", "10")

    @pytest.fixture
    def parallel_algo(self):
        example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order

def handle_data(asset: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
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
        assert parallel_algo._file_path is not None
        assert parallel_algo._functions is not None
        assert len(parallel_algo._functions) == 1
        assert "handle_data" in parallel_algo._functions
        assert parallel_algo._functions["handle_data"] == {
            "callable": mock.ANY,
            "asset_key": "asset",
            "portfolio_key": "portfolio",
            "definition": worker_pb2.Algorithm.Function(
                name="handle_data",
                parameters=[
                    worker_pb2.Algorithm.FunctionParameter(
                        key="low",
                        defaultValue="5",
                        valueType="int",
                    ),
                    worker_pb2.Algorithm.FunctionParameter(
                        key="high",
                        defaultValue="10",
                        valueType="int",
                    ),
                ],
                parallelExecution=True,
                runFirst=False,
                runLast=False,
            ),
        }
        assert parallel_algo._namespaces == []
        parallel_algo.configure("handle_data", "low", "5")
        parallel_algo.configure("handle_data", "high", "10")

    @pytest.fixture
    def algo_with_namespace(self):
        example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order

def handle_data(asset: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
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

    def test_algo_with_namespace(self, algo_with_namespace: Algorithm):
        assert algo_with_namespace._file_path is not None
        assert algo_with_namespace._functions is not None
        assert len(algo_with_namespace._functions) == 1
        assert "handle_data" in algo_with_namespace._functions
        assert algo_with_namespace._functions["handle_data"] == {
            "callable": mock.ANY,
            "asset_key": "asset",
            "portfolio_key": "portfolio",
            "definition": worker_pb2.Algorithm.Function(
                name="handle_data",
                parameters=[
                    worker_pb2.Algorithm.FunctionParameter(
                        key="low",
                        defaultValue="5",
                        valueType="int",
                    ),
                    worker_pb2.Algorithm.FunctionParameter(
                        key="high",
                        defaultValue="10",
                        valueType="int",
                    ),
                ],
                parallelExecution=True,
                runFirst=False,
                runLast=False,
            ),
        }
        assert algo_with_namespace._namespaces == ["qualified_symbols", "rsi"]
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

    def test_multistep_with_namespace(self, multistep_algo_with_namespace: Algorithm):
        assert multistep_algo_with_namespace._file_path is not None
        assert multistep_algo_with_namespace._functions is not None
        assert len(multistep_algo_with_namespace._functions) == 3
        assert "measure_assets" in multistep_algo_with_namespace._functions
        assert multistep_algo_with_namespace._functions["measure_assets"] == {
            "callable": mock.ANY,
            "asset_key": "asset",
            "portfolio_key": None,
            "definition": worker_pb2.Algorithm.Function(
                name="measure_assets",
                parameters=[
                    worker_pb2.Algorithm.FunctionParameter(
                        key="low",
                        defaultValue="5",
                        valueType="int",
                    ),
                    worker_pb2.Algorithm.FunctionParameter(
                        key="high",
                        defaultValue="10",
                        valueType="int",
                    ),
                ],
                parallelExecution=True,
                runFirst=False,
                runLast=False,
            ),
        }
        assert "create_orders" in multistep_algo_with_namespace._functions
        assert multistep_algo_with_namespace._functions["create_orders"] == {
            "callable": mock.ANY,
            "asset_key": "assets",
            "portfolio_key": "portfolio",
            "definition": worker_pb2.Algorithm.Function(
                name="create_orders",
                parameters=[],
                parallelExecution=False,
                runFirst=False,
                runLast=True,
            ),
        }
        assert "filter_assets" in multistep_algo_with_namespace._functions
        assert multistep_algo_with_namespace._functions["filter_assets"] == {
            "callable": mock.ANY,
            "asset_key": "assets",
            "portfolio_key": None,
            "definition": worker_pb2.Algorithm.Function(
                name="filter_assets",
                parameters=[],
                parallelExecution=False,
                runFirst=True,
                runLast=False,
            ),
        }
        assert multistep_algo_with_namespace._namespaces == [
            "qualified_symbols",
            "asset_metrics",
        ]
        multistep_algo_with_namespace.configure("measure_assets", "low", "5")
        multistep_algo_with_namespace.configure("measure_assets", "high", "10")
