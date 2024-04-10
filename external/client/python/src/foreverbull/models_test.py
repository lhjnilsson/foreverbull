import tempfile

import pytest

from foreverbull.models import Algorithm, Namespace


def test_namespace():
    n = Namespace(key1=dict[str, int], key2=list[float])
    assert n.contains("key1", dict[str, int])
    assert n.contains("key2", list[float])
    with pytest.raises(KeyError):
        n.contains("key3", dict[str, int])


"""
namespace = {
    "qualified_symbols": {
        "type": "array",
        "items": {
            "type": "string"
        }
    },
    "rsi": {
        "type": "object",
        "items": {
            "type": "number"
        }
    }
}

"""

SequentialAlgo = b"""
from foreverbull import Algorithm, Function
from foreverbull.data import Asset
from foreverbull.entity.finance import Portfolio, Order

def handle_data(low: int, high: int, assets: list[Asset], portfolio: Portfolio) -> list[Order]:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
ParallelAlgo = b"""
from foreverbull import Algorithm, Function
from foreverbull.data import Asset
from foreverbull.entity.finance import Portfolio, Order

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""

AlgoWithNamespace = b"""
from foreverbull import Algorithm, Function, Namespace
from foreverbull.data import Asset
from foreverbull.entity.finance import Portfolio, Order

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ],
    namespace=Namespace(qualified_symbols=list[str], rsi=dict[str, float])
)
"""

MultiStepAlgoWithNamespace = b"""
from foreverbull import Algorithm, Function, Namespace
from foreverbull.data import Asset
from foreverbull.entity.finance import Portfolio, Order

def filter_assets(assets: list[Asset]) -> list[str]:
    pass

def measure_assets(asses: Asset, low: int = 5, high: int = 10) -> dict[str, float]:
    pass
    
def create_orders(assets: list[Asset], portfolio: Portfolio) -> list[Order]:
    pass

Algorithm(
    functions=[
        Function(callable=filter_assets, namespace_return_key="qualified_symbols"),
        Function(callable=measure_assets, namespace_return_key="asset_metrics", input_key="qualified_symbols"),
        Function(callable=create_orders, input_key="qualified_symbols")
    ],
    namespace=Namespace(
        qualified_symbols=list[str],
        asset_metrics=dict[str, float]
    )
)
"""


def test_sequential_algo():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(SequentialAlgo)
        f.flush()
        algo = Algorithm.from_file_path(f.name)
        assert algo.functions is not None
        assert len(algo.functions) == 1
        assert algo.functions[0].name == "handle_data"
        assert len(algo.functions[0].parameters) == 2
        assert algo.functions[0].style == "SEQUENTIAL"
        assert algo.namespace == {}


def test_parallel_algo():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(ParallelAlgo)
        f.flush()
        algo = Algorithm.from_file_path(f.name)
        assert algo.functions is not None
        assert len(algo.functions) == 1
        assert algo.functions[0].name == "handle_data"
        assert len(algo.functions[0].parameters) == 2
        assert algo.functions[0].style == "PARALLEL"
        assert algo.namespace == {}


def test_algo_with_namespace():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(AlgoWithNamespace)
        f.flush()
        algo = Algorithm.from_file_path(f.name)
        assert algo.functions is not None
        assert len(algo.functions) == 1
        assert algo.functions[0].name == "handle_data"
        assert len(algo.functions[0].parameters) == 2
        assert algo.functions[0].style == "PARALLEL"
        assert algo.namespace is not None
        assert algo.namespace.contains("qualified_symbols", list[str])
        assert algo.namespace.contains("rsi", dict[str, float])
        assert algo.entity is not None
        assert algo.entity.namespace == {
            "qualified_symbols": {"items": {"type": "str"}, "type": "array"},
            "rsi": {"items": {"type": "float"}, "type": "object"},
        }


def test_multi_step_algo_with_namespace():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(MultiStepAlgoWithNamespace)
        f.flush()
        algo = Algorithm.from_file_path(f.name)
        assert algo.functions is not None
        assert len(algo.functions) == 3

        assert algo.functions[0].name == "filter_assets"
        assert len(algo.functions[0].parameters) == 0
        assert algo.functions[0].style == "SEQUENTIAL"

        assert algo.functions[1].name == "measure_assets"
        assert len(algo.functions[1].parameters) == 2
        assert algo.functions[1].style == "PARALLEL"

        assert algo.functions[2].name == "create_orders"
        assert len(algo.functions[2].parameters) == 0
        assert algo.functions[2].style == "SEQUENTIAL"

        assert algo.namespace is not None
        assert algo.namespace.contains("qualified_symbols", list[str])
        assert algo.namespace.contains("asset_metrics", dict[str, float])
        assert algo.entity is not None
        assert algo.entity.namespace is not None
        assert algo.entity.namespace == {
            "qualified_symbols": {"items": {"type": "str"}, "type": "array"},
            "asset_metrics": {"items": {"type": "float"}, "type": "object"},
        }
