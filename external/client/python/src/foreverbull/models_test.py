import tempfile

import pytest

from foreverbull import entity
from foreverbull.models import Algorithm, Namespace


def test_namespace():
    n = Namespace(key1=dict[str, int], key2=list[float])
    assert n.contains("key1", dict[str, int])
    assert n.contains("key2", list[float])
    with pytest.raises(KeyError):
        n.contains("key3", dict[str, int])


class TestNonParallel:
    example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order

def handle_data(low: int, high: int, assets: list[Asset], portfolio: Portfolio) -> list[Order]:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""

    @pytest.fixture
    def algo(self):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)

    def test_entity(self, algo):
        entity = self.algo.get_entity()
        assert entity.functions is not None
        assert len(entity.functions) == 1
        assert entity.functions[0].name == "handle_data"
        assert len(entity.functions[0].parameters) == 2
        assert entity.functions[0].parallel_execution is False

    def test_configure_and_process(self, algo):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "handle_data": entity.service.Execution.Function(
                    parameters={"low": "5", "high": "10"},
                )
            },
        )
        self.algo.configure(execution)

        self.algo.process("handle_data", [], None)


class TestParallel:
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

    @pytest.fixture
    def algo(self):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)

    def test_entity(self, algo):
        entity = self.algo.get_entity()
        assert entity.functions is not None
        assert len(entity.functions) == 1
        assert entity.functions[0].name == "handle_data"
        assert len(entity.functions[0].parameters) == 2
        assert entity.functions[0].parallel_execution is True

    def test_configure_and_process(self, algo):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "handle_data": entity.service.Execution.Function(
                    parameters={"low": "5", "high": "10"},
                )
            },
        )
        self.algo.configure(execution)

        self.algo.process("handle_data", [], None)


class TestWithNamespace:
    example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order, Namespace

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ],
    namespace=Namespace(qualified_symbols=list[str], rsi=dict[str, float])
)
"""

    @pytest.fixture
    def algo(self):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)

    def test_entity(self, algo):
        entity = self.algo.get_entity()
        assert entity.functions is not None
        assert len(entity.functions) == 1
        assert entity.functions[0].name == "handle_data"
        assert len(entity.functions[0].parameters) == 2
        assert entity.functions[0].parallel_execution is True
        assert entity.namespace is not None
        assert "qualified_symbols" in entity.namespace
        assert entity.namespace["qualified_symbols"] == {"type": "array", "items": {"type": "str"}}
        assert "rsi" in entity.namespace
        assert entity.namespace["rsi"] == {"type": "object", "items": {"type": "float"}}

    def test_configure_and_process(self, algo):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "handle_data": entity.service.Execution.Function(
                    parameters={"low": "5", "high": "10"},
                )
            },
        )
        self.algo.configure(execution)

        self.algo.process("handle_data", [], None)


class TestMultiStepWithNamespace:
    example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order, Namespace

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

    @pytest.fixture
    def algo(self):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)

    def test_entity(self, algo):
        entity = self.algo.get_entity()
        assert entity.functions is not None
        assert len(entity.functions) == 3

        assert entity.functions[0].name == "filter_assets"
        assert len(entity.functions[0].parameters) == 0
        assert entity.functions[0].parallel_execution is False

        assert entity.functions[1].name == "measure_assets"
        assert len(entity.functions[1].parameters) == 2
        assert entity.functions[1].parallel_execution is True

        assert entity.functions[2].name == "create_orders"
        assert len(entity.functions[2].parameters) == 0
        assert entity.functions[2].parallel_execution is False

        assert entity.namespace is not None
        assert "qualified_symbols" in entity.namespace
        assert entity.namespace["qualified_symbols"] == {"type": "array", "items": {"type": "str"}}
        assert "asset_metrics" in entity.namespace
        assert entity.namespace["asset_metrics"] == {"type": "object", "items": {"type": "float"}}

    def test_configure_and_process(self, algo):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "filter_assets": entity.service.Execution.Function(),
                "measure_assets": entity.service.Execution.Function(
                    parameters={"low": "5", "high": "10"},
                ),
                "create_orders": entity.service.Execution.Function(),
            },
        )
        self.algo.configure(execution)

        self.algo.process("filter_assets", [], None)
        self.algo.process("measure_assets", [], None)
        self.algo.process("create_orders", [], None)
