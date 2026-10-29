import tempfile

from datetime import UTC
from datetime import datetime
from datetime import timedelta
from typing import Any
from typing import cast
from unittest import mock

import pandas
import pytest

from protobuf import merge_from_binary

from foreverbull.exceptions import ConfigurationError
from foreverbull.models import Algorithm
from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.models import Function
from foreverbull.models import Portfolio
from foreverbull.models import deployable_value
from foreverbull.models import equal_weight_percent
from foreverbull.models.finance import _STOCK_DATA_LOOKBACK
from foreverbull.models.service import _eval_param
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.service.v1 import worker_pb


def _bar(symbol: str, when: datetime, close: float, *, volume: int = 100) -> marketdata_pb.OHLC:
    return marketdata_pb.OHLC(
        symbol=symbol,
        timestamp=pb_utils.to_proto_timestamp(when),
        open=close,
        high=close + 1,
        low=close - 1,
        close=close,
        volume=volume,
    )


class TestAsset:
    def test_asset_getattr_setattr(self, fb_env, namespace_server):
        asset = Asset(datetime.now(), "AAPL")
        assert asset is not None
        asset.set_metric("rsi", 56.4)

        assert "rsi" in namespace_server
        assert namespace_server["rsi"] == {"AAPL": 56.4}

        namespace_server["pe"] = {"AAPL": 12.3}
        assert asset.get_metric("pe") == 12.3

    def test_stock_data_uses_bars_for_the_symbol(self):
        as_of = datetime(2024, 6, 3, tzinfo=UTC)
        bars = [_bar("AAPL", as_of - timedelta(days=i), 100 + i) for i in range(3)]
        bars.append(_bar("MSFT", as_of, 50))
        asset = Asset(as_of, "AAPL", bars)
        stock_data = asset.stock_data
        assert list(stock_data.columns) == ["symbol", "time", "high", "low", "open", "close", "volume"]
        assert stock_data["symbol"].tolist() == ["AAPL", "AAPL", "AAPL"]
        assert stock_data["close"].tolist() == [102, 101, 100]
        assert len(stock_data) <= _STOCK_DATA_LOOKBACK


class TestAssets:
    def test_assets_getattr_setattr(self, fb_env, namespace_server):
        assets = Assets(datetime.now(), [])
        assert assets is not None
        assets.set_metrics("holdings", {"AAPL": True, "MSFT": False})

        assert "holdings" in namespace_server
        assert namespace_server["holdings"] == {"AAPL": True, "MSFT": False}

        namespace_server["pe"] = {"AAPL": 12.3, "MSFT": 23.4}
        assert assets.get_metrics("pe").to_dict() == {"AAPL": 12.3, "MSFT": 23.4}

    def test_stock_data(self):
        as_of = datetime(2024, 6, 3, tzinfo=UTC)
        bars = [
            _bar("AAPL", as_of - timedelta(days=1), 10),
            _bar("MSFT", as_of, 20),
            _bar("AAPL", as_of, 11),
        ]
        stock_data = Assets(as_of, ["AAPL", "MSFT"], bars).stock_data
        assert list(stock_data.index.names) == ["symbol", "time"]
        assert set(stock_data.columns) == {"high", "low", "open", "close", "volume"}
        counts = stock_data.groupby(level="symbol").size()
        assert counts.to_dict() == {"AAPL": 2, "MSFT": 1}
        assert (counts <= _STOCK_DATA_LOOKBACK).all()

    def test_iterated_stock_data_matches_direct_asset(self):
        as_of = datetime(2024, 6, 3, tzinfo=UTC)
        symbols = ["AAPL", "MSFT"]
        bars = [_bar(symbol, as_of, close) for symbol, close in (("AAPL", 10), ("MSFT", 20))]
        assets = Assets(as_of, symbols, bars)
        for asset in assets:
            direct = Asset(as_of, asset.symbol, bars).stock_data
            pandas.testing.assert_frame_equal(asset.stock_data, direct, check_dtype=True)
            pandas.testing.assert_frame_equal(asset.stock_data, direct, check_dtype=True)

    def test_assets_stock_data_one_symbol_matches_asset(self):
        as_of = datetime(2024, 6, 3, tzinfo=UTC)
        symbols = ["AAPL", "MSFT"]
        bars = [_bar("AAPL", as_of, 10), _bar("MSFT", as_of, 20)]
        batched = Assets(as_of, symbols, bars).stock_data
        direct = Asset(as_of, "AAPL", bars).stock_data
        one = batched.loc[["AAPL"]].reset_index()
        pandas.testing.assert_frame_equal(one, direct, check_dtype=True)


_PORTFOLIO_DAY = datetime(2023, 12, 29, tzinfo=UTC)
_PORTFOLIO_CLOSE = 193


class TestPortfolio:
    @pytest.fixture
    def uut(self):
        pb = marketdata_pb.Portfolio(
            timestamp=pb_utils.to_proto_timestamp(_PORTFOLIO_DAY),
            portfolio_value=1000000,
        )
        return Portfolio(pb, [_bar("ptest", _PORTFOLIO_DAY, _PORTFOLIO_CLOSE)])

    def test_calculate_order_value_amount(self, uut: Portfolio):
        close = uut._calculate_order_value_amount("ptest", 100000)
        assert close
        assert close == 518

    def test_calculate_order_value_amount_refuses_negative_value_and_negative_close(self, uut: Portfolio):
        with pytest.raises(ConfigurationError, match="order value is negative"):
            uut._calculate_order_value_amount("ptest", -190)
        with pytest.raises(ConfigurationError, match="order value is negative"):
            uut.order_value("ptest", -190)
        with pytest.raises(ConfigurationError, match="order value is negative"):
            uut.order_target_value("ptest", -190)
        assert uut.pending_orders == []

        uut._bars = [_bar("ptest", _PORTFOLIO_DAY, -50)]
        uut._close_by_symbol_day.clear()
        with pytest.raises(ConfigurationError, match="close is negative"):
            uut.order_value("ptest", 190)
        with pytest.raises(ConfigurationError, match="close is negative"):
            uut.order_target_value("ptest", 10000)
        assert uut.pending_orders == []

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
            position = marketdata_pb.Position(
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
            position = marketdata_pb.Position(
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
            position = marketdata_pb.Position(
                symbol="ptest",
                amount=position_amount,
            )
            uut._pb.positions.append(position)

        order = uut.order_target_value("ptest", order_target_amount)
        assert order
        assert order.amount == expected_order_amount

    @pytest.mark.parametrize("percent", [0.10, 0.5, 1.0])
    def test_order_share_counts_follow_portfolio_value_percent_over_close(self, uut: Portfolio, percent: float):
        close = _PORTFOLIO_CLOSE
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
        assert uut._close_by_symbol_day == {}
        first = uut.order_value("ptest", 50000)
        assert len(uut._close_by_symbol_day) == 1
        second = uut.order_value("ptest", 50000)
        other = uut.order_value("ptest", 100000)
        assert first.amount == second.amount == int(50000 / _PORTFOLIO_CLOSE)
        assert other.amount == int(100000 / _PORTFOLIO_CLOSE)
        assert len(uut._close_by_symbol_day) == 1


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
        portfolio = Portfolio(marketdata_pb.Portfolio(portfolio_value=100_000))
        with pytest.raises(ConfigurationError, match="cash reserve outside"):
            portfolio.equal_weight_percent(1, cash_reserve, 1)

    def test_deployable_value(self):
        assert deployable_value(100_000, 0.05) == 95_000
        assert deployable_value(100_000, 0.0) == 100_000
        assert deployable_value(100_000, 1.0) == 0.0

    def test_portfolio_helpers_match_module(self):
        portfolio = Portfolio(marketdata_pb.Portfolio(portfolio_value=200_000))
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
    algo.configure("handle", "name", "hello")
    configured = algo._functions["handle"]["callable"]
    assert configured.keywords == {"threshold": 2.5, "enabled": False, "name": "hello"}
    with pytest.raises(TypeError, match="Unknown parameter: missing"):
        algo.configure("handle", "missing", "1")
    with pytest.raises(ValueError, match="Couldn't parse bool: use true/false"):
        algo.configure("handle", "enabled", "maybe")


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("True", True),
        ("true", True),
        ("1", True),
        (" yes ", True),
        ("False", False),
        ("false", False),
        ("0", False),
        ("no", False),
    ],
)
def test_eval_param_bool_parses_strings(raw: str, expected: bool):
    assert _eval_param("bool", raw) is expected


def test_eval_param_string_accepts_wire_and_python_labels():
    assert _eval_param("string", "hello") == "hello"
    assert _eval_param("str", "hello") == "hello"
    with pytest.raises(TypeError, match="Unknown parameter type: list"):
        _eval_param("list", "x")


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
    parsed = worker_pb.Algorithm()
    merge_from_binary(parsed, definition.to_binary())
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
    portfolio = marketdata_pb.Portfolio(
        timestamp=pb_utils.to_proto_timestamp(datetime(2024, 3, 9, tzinfo=UTC)),
        cash=1000,
    )

    batched = sequential.process("handle_data", portfolio, ["AAPL", "MSFT"])
    assert [(order.symbol, order.amount) for order in batched] == [("AAPL", 7), ("MSFT", 7)]

    per_symbol = parallel.process("measure_assets", portfolio, ["AAPL", "MSFT"])
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
            "definition": worker_pb.Algorithm.Function(
                name="handle_data",
                parameters=[
                    worker_pb.Algorithm.FunctionParameter(
                        key="low",
                        valueType="int",
                    ),
                    worker_pb.Algorithm.FunctionParameter(
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
            "definition": worker_pb.Algorithm.Function(
                name="handle_data",
                parameters=[
                    worker_pb.Algorithm.FunctionParameter(
                        key="low",
                        defaultValue="5",
                        valueType="int",
                    ),
                    worker_pb.Algorithm.FunctionParameter(
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
            "definition": worker_pb.Algorithm.Function(
                name="handle_data",
                parameters=[
                    worker_pb.Algorithm.FunctionParameter(
                        key="low",
                        defaultValue="5",
                        valueType="int",
                    ),
                    worker_pb.Algorithm.FunctionParameter(
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
            "definition": worker_pb.Algorithm.Function(
                name="measure_assets",
                parameters=[
                    worker_pb.Algorithm.FunctionParameter(
                        key="low",
                        defaultValue="5",
                        valueType="int",
                    ),
                    worker_pb.Algorithm.FunctionParameter(
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
            "definition": worker_pb.Algorithm.Function(
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
            "definition": worker_pb.Algorithm.Function(
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


def test_missing_prices_line_names_the_day_or_the_span():
    from datetime import date

    from foreverbull.models.finance import missing_prices_line

    assert (
        missing_prices_line("XYZ", [date(2026, 5, 2)])
        == "Missing prices for XYZ on 2026-05-02, filled with the previous close. No trades on those days."
    )
    days = [date(2026, 5, 7), date(2026, 5, 4), date(2026, 5, 5), date(2026, 5, 6)]
    assert (
        missing_prices_line("XYZ", days)
        == "Missing prices for XYZ on 4 days (2026-05-04 to 2026-05-07), filled with the previous close. No trades on those days."
    )


def test_missing_prices_summary_folds_three_or_more_symbols():
    from foreverbull.models.finance import missing_prices_summary

    two = ["a", "b"]
    assert missing_prices_summary(two, verbose=False) == two
    many = [f"line {index}" for index in range(12)]
    assert missing_prices_summary(many, verbose=False) == [
        "Missing prices for 12 symbols, filled with the previous close. No trades on those days. Run with --verbose for the list."
    ]
    assert missing_prices_summary(many, verbose=True) == [
        "Missing prices for 12 symbols, filled with the previous close. No trades on those days.",
        *many,
    ]
