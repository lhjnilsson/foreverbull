import tempfile

from datetime import UTC
from datetime import datetime
from typing import Any
from typing import cast
from unittest import mock

import pandas
import pytest

from sqlalchemy import event
from sqlalchemy import text
from sqlalchemy.engine import Connection
from sqlalchemy.exc import IntegrityError

from foreverbull.broker.utils import database_url_for_host
from foreverbull.exceptions import ConfigurationError
from foreverbull.exceptions import ConnectionError
from foreverbull.models import Algorithm
from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.models import Function
from foreverbull.models import Portfolio
from foreverbull.models import _get_postgres_urls
from foreverbull.models import deployable_value
from foreverbull.models import equal_weight_percent
from foreverbull.models import get_engine
from foreverbull.models.finance import _STOCK_DATA_LOOKBACK
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2


@pytest.mark.parametrize(
    "input,expected_output",
    [
        (
            "postgres://user:password@127.0.0.1:5656/demo",
            ["postgresql://user:password@127.0.0.1:5656/demo"],
        ),
        (
            "postgresql://user:password@db.internal:5433/demo",
            ["postgresql://user:password@db.internal:5433/demo"],
        ),
        (
            "postgres://user:password@postgres:5432/demo",
            ["postgresql://user:password@postgres:5432/demo"],
        ),
    ],
)
def test_get_postgres_urls(input, expected_output):
    assert [u for u in _get_postgres_urls(input)] == expected_output


def test_database_url_for_host_uses_kind_localhost(monkeypatch):
    monkeypatch.delenv("BROKER_HOSTNAME", raising=False)
    monkeypatch.setattr(
        "foreverbull.broker.utils.load_cluster_config",
        lambda **kwargs: {"cluster": "local"},
    )
    url = database_url_for_host("postgres://foreverbull:foreverbull@postgres:5432/foreverbull")
    assert url == "postgresql://foreverbull:foreverbull@127.0.0.1:5432/foreverbull"


def test_database_url_for_host_keeps_remote_and_incluster_urls(monkeypatch):
    monkeypatch.setattr(
        "foreverbull.broker.utils.load_cluster_config",
        lambda **kwargs: {"cluster": "remote"},
    )
    remote = "postgres://foreverbull:foreverbull@postgres:5432/foreverbull"
    assert database_url_for_host(remote) == remote

    monkeypatch.setenv("BROKER_HOSTNAME", "foreverbull")
    monkeypatch.setattr(
        "foreverbull.broker.utils.load_cluster_config",
        lambda **kwargs: {"cluster": "local"},
    )
    assert database_url_for_host(remote) == remote
    assert (
        database_url_for_host("postgres://foreverbull:foreverbull@127.0.0.1:5432/foreverbull")
        == "postgres://foreverbull:foreverbull@127.0.0.1:5432/foreverbull"
    )


def test_get_engine_uses_provided_url_only(monkeypatch):
    attempted = []

    def fake_create_engine(url):
        attempted.append(url)
        raise OSError("refused")

    monkeypatch.setattr("foreverbull.broker.utils.create_engine", fake_create_engine)
    with pytest.raises(
        ConnectionError,
        match="Could not connect to database at postgresql://user:password@db.internal:5656/demo",
    ):
        get_engine("postgres://user:password@db.internal:5656/demo")
    assert attempted == ["postgresql://user:password@db.internal:5656/demo"]


class TestAsset:
    def test_asset_getattr_setattr(self, fb_env, namespace_server):
        asset = Asset(datetime.now(), cast(Connection, None), "AAPL")
        assert asset is not None
        asset.set_metric("rsi", 56.4)

        assert "rsi" in namespace_server
        assert namespace_server["rsi"] == {"AAPL": 56.4}

        namespace_server["pe"] = {"AAPL": 12.3}
        assert asset.get_metric("pe") == 12.3

    def test_assets(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        with database.connect() as conn:
            symbols = [asset.symbol for asset in backtest_entity.periods[0].assets if asset.enabled]
            assets = Assets(datetime.now(), conn, symbols)
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
                assert len(stock_data) <= _STOCK_DATA_LOOKBACK

    def test_stock_data_uses_bound_parameters(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        with database.connect() as conn:
            before = conn.execute(text("SELECT COUNT(*) FROM ohlc")).scalar()
            asset = Asset(datetime.now(), conn, "AAPL'; DROP TABLE ohlc; --")
            stock_data = asset.stock_data
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) == 0
            after = conn.execute(text("SELECT COUNT(*) FROM ohlc")).scalar()
            assert after == before


class TestAssets:
    def test_assets_getattr_setattr(self, fb_env, namespace_server):
        assets = Assets(datetime.now(), cast(Connection, None), [])
        assert assets is not None
        assets.set_metrics("holdings", {"AAPL": True, "MSFT": False})

        assert "holdings" in namespace_server
        assert namespace_server["holdings"] == {"AAPL": True, "MSFT": False}

        namespace_server["pe"] = {"AAPL": 12.3, "MSFT": 23.4}
        assert assets.get_metrics("pe").to_dict() == {"AAPL": 12.3, "MSFT": 23.4}

    def test_stock_data(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        with database.connect() as conn:
            symbols = [asset.symbol for asset in backtest_entity.periods[0].assets if asset.enabled]
            assets = Assets(datetime.now(), conn, symbols)
            stock_data = assets.stock_data
            assert stock_data is not None
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) > 0
            assert list(stock_data.index.names) == ["symbol", "time"]
            assert "open" in stock_data.columns
            assert "high" in stock_data.columns
            assert "low" in stock_data.columns
            assert "close" in stock_data.columns
            assert "volume" in stock_data.columns
            counts = stock_data.groupby(level="symbol").size()
            assert (counts <= _STOCK_DATA_LOOKBACK).all()

    def test_stock_data_uses_bound_parameters(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        with database.connect() as conn:
            before = conn.execute(text("SELECT COUNT(*) FROM ohlc")).scalar()
            assets = Assets(datetime.now(), conn, ["AAPL'; DROP TABLE ohlc; --", "MSFT"])
            stock_data = assets.stock_data
            assert isinstance(stock_data, pandas.DataFrame)
            after = conn.execute(text("SELECT COUNT(*) FROM ohlc")).scalar()
            assert after == before
            assert "AAPL'; DROP TABLE ohlc; --" not in (
                stock_data.index.get_level_values("symbol") if not stock_data.empty else []
            )

    def test_ohlc_rejects_a_second_bar_at_the_same_time(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        with database.connect() as conn:
            row = conn.execute(text("SELECT symbol, time FROM ohlc LIMIT 1")).one()
            with pytest.raises(IntegrityError):
                conn.execute(
                    text(
                        """INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
                        VALUES (:symbol, :time, 1, 1, 1, 1, 1)"""
                    ),
                    {"symbol": row.symbol, "time": row.time},
                )
            conn.rollback()

    def test_iterated_stock_data_matches_direct_asset(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        as_of = datetime.now()
        with database.connect() as conn:
            symbols = [asset.symbol for asset in backtest_entity.periods[0].assets if asset.enabled]
            assert symbols
            assets = Assets(as_of, conn, symbols)
            for asset in assets:
                direct = Asset(as_of, conn, asset.symbol).stock_data
                pandas.testing.assert_frame_equal(asset.stock_data, direct, check_dtype=True)

    def test_assets_stock_data_one_symbol_matches_asset_query(
        self, fb_database, backtest_entity: backtest_pb2.Backtest
    ):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        as_of = datetime.now()
        with database.connect() as conn:
            symbols = [asset.symbol for asset in backtest_entity.periods[0].assets if asset.enabled]
            symbol = symbols[0]
            batched = Assets(as_of, conn, symbols).stock_data
            direct = Asset(as_of, conn, symbol).stock_data
            # symbol, time are the index on the batched frame; Asset keeps them as columns.
            one = batched.loc[[symbol]].reset_index()
            pandas.testing.assert_frame_equal(one, direct, check_dtype=True)

    def test_iterated_stock_data_is_one_statement(self, fb_database, backtest_entity: backtest_pb2.Backtest):
        database, ensure_data = fb_database
        ensure_data(backtest_entity)
        as_of = datetime.now()
        with database.connect() as conn:
            symbols = [asset.symbol for asset in backtest_entity.periods[0].assets if asset.enabled]
            assert len(symbols) >= 2
            executions: list[str] = []

            def before_cursor_execute(conn, cursor, statement, parameters, context, executemany):
                executions.append(statement)

            event.listen(conn, "before_cursor_execute", before_cursor_execute)
            try:
                assets = Assets(as_of, conn, symbols)
                frames = []
                for asset in assets:
                    frames.append(asset.stock_data)
                    frames.append(asset.stock_data)
                assert len(frames) == len(symbols) * 2
                assert len(executions) == 1
                assert "row_number" in executions[0].lower()

                direct = Asset(as_of, conn, symbols[0]).stock_data
                assert isinstance(direct, pandas.DataFrame)
                assert len(executions) == 2
                assert "row_number" not in executions[1].lower()
            finally:
                event.remove(conn, "before_cursor_execute", before_cursor_execute)


class TestPortfolio:
    @pytest.fixture(scope="session")
    def db(self, fb_database):
        database, _ = fb_database
        with database.connect() as conn:
            conn.execute(
                text(
                    """INSERT INTO asset (symbol, name)
                    VALUES (:symbol, :name) ON CONFLICT DO NOTHING"""
                ),
                {"symbol": "ptest", "name": "PortFolio Test"},
            )
            conn.execute(
                text(
                    """INSERT INTO ohlc (symbol, open, high, low, close, volume, time)
                    VALUES (:symbol, :open, :high, :low, :close, :volume, :time) ON CONFLICT DO NOTHING"""
                ),
                {
                    "symbol": "ptest",
                    "open": 10,
                    "high": 20,
                    "low": 5,
                    "close": 193,  # We should use Close when calculating order value
                    "volume": 100,
                    "time": datetime(2023, 12, 29, tzinfo=UTC),
                },
            )
            conn.commit()
        yield database

    @pytest.fixture(scope="function")
    def uut(self, db):
        with db.connect() as conn:
            pb = marketdata_pb2.Portfolio(
                timestamp=pb_utils.to_proto_timestamp(datetime(2023, 12, 29, tzinfo=UTC)),
                portfolio_value=1000000,
            )
            portfolio = Portfolio(pb, conn)
            yield portfolio

    def test_calculate_order_value_amount(self, uut: Portfolio):
        close = uut._calculate_order_value_amount("ptest", 100000)
        assert close
        assert close == 518

    def test_order(self, uut: Portfolio):
        order = uut.order("ptest", 100)
        assert order
        assert order.amount == 100

    def test_order_percent(self, uut: Portfolio):
        order = uut.order_percent("ptest", 0.5)
        assert order
        assert order.amount == 2590

    def test_order_value(self, uut: Portfolio):
        order = uut.order_value("ptest", 50000)
        assert order
        assert order.amount == 259

    @pytest.mark.parametrize(
        "position_amount,order_target_amount,expected_order_amount",
        [
            (0, 100, 100),
            (50, 100, 50),
            (100, 50, -50),
            (50, 50, 0),
        ],
    )
    def test_order_target(
        self,
        uut: Portfolio,
        position_amount,
        order_target_amount,
        expected_order_amount,
    ):
        if position_amount:
            position = marketdata_pb2.Position(
                symbol="ptest",
                amount=position_amount,
            )
            uut._pb.positions.append(position)

        order = uut.order_target("ptest", order_target_amount)
        assert order
        assert order.amount == expected_order_amount

    @pytest.mark.parametrize(
        "position_amount,order_target_amount,expected_order_amount",
        [(0, 0.10, 518), (100, 0.10, 418), (600, 0.10, -82)],
    )
    def test_order_target_percent(
        self,
        uut: Portfolio,
        position_amount,
        order_target_amount,
        expected_order_amount,
    ):
        if position_amount:
            position = marketdata_pb2.Position(
                symbol="ptest",
                amount=position_amount,
            )
            uut._pb.positions.append(position)

        order = uut.order_target_percent("ptest", order_target_amount)
        assert order
        assert order.amount == expected_order_amount

    @pytest.mark.parametrize(
        "position_amount,order_target_amount,expected_order_amount",
        [(0, 10000, 51), (10, 10000, 41), (60, 10000, -9)],
    )
    def test_order_target_value(
        self,
        uut: Portfolio,
        position_amount,
        order_target_amount,
        expected_order_amount,
    ):
        if position_amount:
            position = marketdata_pb2.Position(
                symbol="ptest",
                amount=position_amount,
            )
            uut._pb.positions.append(position)

        order = uut.order_target_value("ptest", order_target_amount)
        assert order
        assert order.amount == expected_order_amount

    @pytest.mark.parametrize("percent", [0.10, 0.5, 1.0])
    def test_order_share_counts_follow_portfolio_value_percent_over_close(self, uut: Portfolio, percent: float):
        close = uut._db.execute(
            text("SELECT close FROM ohlc WHERE symbol=:symbol and time::DATE=:dt"),
            {"symbol": "ptest", "dt": uut._pb.timestamp.ToDatetime().strftime("%Y-%m-%d")},
        ).scalar()
        assert close is not None
        expected = int(uut._pb.portfolio_value * percent / float(close))

        by_value = uut.order_value("ptest", uut._pb.portfolio_value * percent)
        by_value_again = uut.order_value("ptest", uut._pb.portfolio_value * percent)
        assert by_value.amount == expected
        assert by_value_again.amount == expected

        by_target = uut.order_target_percent("ptest", percent)
        by_target_again = uut.order_target_percent("ptest", percent)
        assert by_target.amount == expected
        assert by_target_again.amount == expected

    def test_repeated_order_value_reuses_close(self, uut: Portfolio):
        day = uut._pb.timestamp.ToDatetime().strftime("%Y-%m-%d")
        close = uut._db.execute(
            text("SELECT close FROM ohlc WHERE symbol=:symbol and time::DATE=:dt"),
            {"symbol": "ptest", "dt": day},
        ).scalar()
        assert close is not None
        executions: list[str] = []

        def before_cursor_execute(conn, cursor, statement, parameters, context, executemany):
            executions.append(statement)

        event.listen(uut._db, "before_cursor_execute", before_cursor_execute)
        try:
            first = uut.order_value("ptest", 50000)
            second = uut.order_value("ptest", 50000)
            other = uut.order_value("ptest", 100000)
        finally:
            event.remove(uut._db, "before_cursor_execute", before_cursor_execute)

        assert first.amount == second.amount == int(50000 / float(close))
        assert other.amount == int(100000 / float(close))
        assert len(executions) == 1


class TestAllocation:
    @pytest.mark.parametrize(
        "n_selected,cash_reserve,max_positions,expected",
        [
            (10, 0.05, 10, 0.095),
            (3, 0.05, 10, 0.95 / 3),
            (50, 0.10, 10, 0.09),
            (1, 0.0, 10, 1.0),
            (10, 0.0, 10, 0.1),
            (0, 0.05, 10, 0.0),
        ],
    )
    def test_equal_weight_percent(self, n_selected, cash_reserve, max_positions, expected):
        weight = equal_weight_percent(n_selected, cash_reserve, max_positions)
        assert weight == pytest.approx(expected)
        n = min(max_positions, n_selected)
        if n >= 1:
            assert n * weight == pytest.approx(1.0 - cash_reserve)
            assert n * weight <= 1.0 - cash_reserve + 1e-12

    @pytest.mark.parametrize("cash_reserve", [-0.1, 1.0, 1.5, float("nan"), float("inf"), float("-inf")])
    def test_equal_weight_percent_rejects_cash_reserve_outside_unit_interval(self, cash_reserve):
        with pytest.raises(ConfigurationError, match="cash reserve outside"):
            equal_weight_percent(1, cash_reserve, 1)
        portfolio = Portfolio(marketdata_pb2.Portfolio(portfolio_value=100_000), cast(Connection, None))
        with pytest.raises(ConfigurationError, match="cash reserve outside"):
            portfolio.equal_weight_percent(1, cash_reserve, 1)

    def test_deployable_value(self):
        assert deployable_value(100_000, 0.05) == 95_000
        assert deployable_value(100_000, 0.0) == 100_000
        assert deployable_value(100_000, 1.0) == 0.0

    def test_portfolio_helpers_match_module(self):
        portfolio = Portfolio(marketdata_pb2.Portfolio(portfolio_value=200_000), cast(Connection, None))
        assert portfolio.deployable_value(0.10) == deployable_value(200_000, 0.10)
        assert portfolio.equal_weight_percent(10, 0.10, 10) == equal_weight_percent(10, 0.10, 10)
        assert portfolio.equal_weight_percent(0, 0.05, 10) == 0.0


def test_algorithm_parameter_parsing():
    assert Algorithm.type_to_str(cast(Any, int)) == "int"
    assert Algorithm.type_to_str(cast(Any, float)) == "float"
    assert Algorithm.type_to_str(cast(Any, bool)) == "bool"
    assert Algorithm.type_to_str(cast(Any, str)) == "string"
    with pytest.raises(TypeError):
        Algorithm.type_to_str(cast(Any, list))

    def no_asset(x: int):
        pass

    with pytest.raises(TypeError, match="must have a parameter of type Asset or Assets"):
        Algorithm(functions=[Function(callable=no_asset)])

    def handle(asset: Asset, threshold: float = 1.5, enabled: bool = True, name: str = "x"):
        pass

    algo = Algorithm(functions=[Function(callable=handle)])
    definition = algo.get_definition()
    params = {p.key: p.valueType for p in definition.functions[0].parameters}
    assert params == {"threshold": "float", "enabled": "bool", "name": "string"}

    algo.configure("handle", "threshold", "2.5")
    algo.configure("handle", "enabled", "false")
    with pytest.raises(TypeError, match="Unknown parameter: missing"):
        algo.configure("handle", "missing", "1")
    with pytest.raises(TypeError, match="Unknown parameter type: string"):
        algo.configure("handle", "name", "hello")


def test_algorithm_definition_wire_contract():
    def filter_assets(assets: Assets) -> None:
        pass

    def measure_assets(asset: Asset, lookback: int = 20) -> None:
        pass

    def create_orders(assets: Assets, portfolio: Portfolio) -> None:
        pass

    algo = Algorithm(
        functions=[
            Function(callable=filter_assets, run_first=True),
            Function(callable=measure_assets),
            Function(callable=create_orders, run_last=True),
        ],
        namespaces=["rsi"],
    )
    definition = algo.get_definition()
    parsed = worker_pb2.Algorithm()
    parsed.ParseFromString(definition.SerializeToString())
    assert list(parsed.namespaces) == ["rsi"]
    flags = [(fn.name, fn.parallelExecution, fn.runFirst, fn.runLast) for fn in parsed.functions]
    assert flags == [
        ("filter_assets", False, True, False),
        ("measure_assets", True, False, False),
        ("create_orders", False, False, True),
    ]
    assert parsed.functions[1].parameters[0].key == "lookback"
    assert parsed.functions[1].parameters[0].valueType == "int"
    assert parsed.functions[1].parameters[0].defaultValue == "20"


def test_algorithm_process_maps_worker_request_to_orders():
    def handle_data(assets: Assets, portfolio: Portfolio) -> None:
        for symbol in assets.symbols:
            portfolio.order(symbol, 7)

    def measure_assets(asset: Asset, portfolio: Portfolio) -> None:
        portfolio.order(asset.symbol, 3)

    sequential = Algorithm(functions=[Function(callable=handle_data)])
    parallel = Algorithm(functions=[Function(callable=measure_assets)])
    portfolio = marketdata_pb2.Portfolio(
        timestamp=pb_utils.to_proto_timestamp(datetime(2024, 3, 9, tzinfo=UTC)),
        cash=1000,
    )

    batched = sequential.process("handle_data", cast(Connection, None), portfolio, ["AAPL", "MSFT"])
    assert [(order.symbol, order.amount) for order in batched] == [("AAPL", 7), ("MSFT", 7)]

    per_symbol = parallel.process("measure_assets", cast(Connection, None), portfolio, ["AAPL", "MSFT"])
    assert [(order.symbol, order.amount) for order in per_symbol] == [("AAPL", 3), ("MSFT", 3)]


class TestDefinitions:
    @pytest.fixture
    def non_parallel_algo(self):
        example = b"""
from foreverbull import Algorithm, Function, Assets, Portfolio, Order

def handle_data(low: int, high: int, assets: Assets, portfolio: Portfolio) -> list[Order]:
    pass

algo = Algorithm(
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

algo = Algorithm(
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

algo = Algorithm(
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

algo = Algorithm(
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
