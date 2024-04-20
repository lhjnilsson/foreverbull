import tempfile

import pytest

from foreverbull import entity
from foreverbull.models import Algorithm, Namespace


def test_namespace():
    n = Namespace(
        key1=dict[
            str,
            int,
        ],
        key2=list[float],
    )
    assert n.contains(
        "key1",
        dict[
            str,
            int,
        ],
    )
    assert n.contains(
        "key2",
        list[float],
    )
    with pytest.raises(KeyError):
        n.contains(
            "key3",
            dict[
                str,
                int,
            ],
        )


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
    def algo(
        self,
    ):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(
        self,
        algo,
    ):
        assert self.algo.get_entity() == entity.service.Algorithm(
            file_path=self.file_path,
            functions=[
                entity.service.Algorithm.Function(
                    name="handle_data",
                    parameters=[
                        entity.service.Algorithm.FunctionParameter(
                            key="low",
                            type="int",
                        ),
                        entity.service.Algorithm.FunctionParameter(
                            key="high",
                            type="int",
                        ),
                    ],
                    parallel_execution=False,
                    return_type=entity.service.Algorithm.ReturnType.LIST_OF_ORDERS,
                    input_key="symbols",
                    namespace_return_key=None,
                ),
            ],
        )

    def test_configure_and_process(
        self,
        algo,
    ):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "handle_data": entity.service.Execution.Function(
                    parameters={
                        "low": "5",
                        "high": "10",
                    },
                )
            },
        )
        self.algo.configure(execution)

        self.algo.process(
            "handle_data",
            [],
            None,
        )


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
    def algo(
        self,
    ):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(
        self,
        algo,
    ):
        assert self.algo.get_entity() == entity.service.Algorithm(
            file_path=self.file_path,
            functions=[
                entity.service.Algorithm.Function(
                    name="handle_data",
                    parameters=[
                        entity.service.Algorithm.FunctionParameter(
                            key="low",
                            default="5",
                            type="int",
                        ),
                        entity.service.Algorithm.FunctionParameter(
                            key="high",
                            default="10",
                            type="int",
                        ),
                    ],
                    parallel_execution=True,
                    return_type=entity.service.Algorithm.ReturnType.ORDER,
                    input_key="symbols",
                    namespace_return_key=None,
                ),
            ],
        )

    def test_configure_and_process(
        self,
        algo,
    ):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "handle_data": entity.service.Execution.Function(
                    parameters={
                        "low": "5",
                        "high": "10",
                    },
                )
            },
        )
        self.algo.configure(execution)

        self.algo.process(
            "handle_data",
            [],
            None,
        )


class TestWithNamespace:
    example = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio, Order, Namespace

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ],
    namespace={"qualified_symbols": list[str], "rsi": dict[str, float]}
)
"""

    @pytest.fixture
    def algo(
        self,
    ):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(
        self,
        algo,
    ):
        assert self.algo.get_entity() == entity.service.Algorithm(
            file_path=self.file_path,
            functions=[
                entity.service.Algorithm.Function(
                    name="handle_data",
                    parameters=[
                        entity.service.Algorithm.FunctionParameter(
                            key="low",
                            default="5",
                            type="int",
                        ),
                        entity.service.Algorithm.FunctionParameter(
                            key="high",
                            default="10",
                            type="int",
                        ),
                    ],
                    parallel_execution=True,
                    return_type=entity.service.Algorithm.ReturnType.ORDER,
                    input_key="symbols",
                    namespace_return_key=None,
                ),
            ],
            namespace={
                "qualified_symbols": entity.service.Algorithm.Namespace(
                    type="array",
                    value_type="string",
                ),
                "rsi": entity.service.Algorithm.Namespace(
                    type="object",
                    value_type="float",
                ),
            },
        )

    def test_configure_and_process(
        self,
        algo,
    ):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "handle_data": entity.service.Execution.Function(
                    parameters={
                        "low": "5",
                        "high": "10",
                    },
                )
            },
        )
        self.algo.configure(execution)

        self.algo.process(
            "handle_data",
            [],
            None,
        )


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
        Function(callable=create_orders, input_key="asset_metrics")
    ],
    namespace={"qualified_symbols": list[str], "asset_metrics": dict[str, float]}
)
"""

    @pytest.fixture
    def algo(
        self,
    ):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self.algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(
        self,
        algo,
    ):
        assert self.algo.get_entity() == entity.service.Algorithm(
            file_path=self.file_path,
            functions=[
                entity.service.Algorithm.Function(
                    name="filter_assets",
                    parameters=[],
                    parallel_execution=False,
                    return_type=entity.service.Algorithm.ReturnType.NAMESPACE_VALUE,
                    input_key="symbols",
                    namespace_return_key="qualified_symbols",
                ),
                entity.service.Algorithm.Function(
                    name="measure_assets",
                    parameters=[
                        entity.service.Algorithm.FunctionParameter(
                            key="low",
                            default="5",
                            type="int",
                        ),
                        entity.service.Algorithm.FunctionParameter(
                            key="high",
                            default="10",
                            type="int",
                        ),
                    ],
                    parallel_execution=True,
                    return_type=entity.service.Algorithm.ReturnType.NAMESPACE_VALUE,
                    input_key="qualified_symbols",
                    namespace_return_key="asset_metrics",
                ),
                entity.service.Algorithm.Function(
                    name="create_orders",
                    parameters=[],
                    parallel_execution=False,
                    return_type=entity.service.Algorithm.ReturnType.LIST_OF_ORDERS,
                    input_key="asset_metrics",
                    namespace_return_key=None,
                ),
            ],
            namespace={
                "qualified_symbols": entity.service.Algorithm.Namespace(
                    type="array",
                    value_type="string",
                ),
                "asset_metrics": entity.service.Algorithm.Namespace(
                    type="object",
                    value_type="float",
                ),
            },
        )

    def test_configure_and_process(
        self,
        algo,
    ):
        execution = entity.service.Execution(
            id="123",
            port=5656,
            database_url="not_used",
            configuration={
                "filter_assets": entity.service.Execution.Function(),
                "measure_assets": entity.service.Execution.Function(
                    parameters={
                        "low": "5",
                        "high": "10",
                    },
                ),
                "create_orders": entity.service.Execution.Function(),
            },
        )
        self.algo.configure(execution)

        self.algo.process(
            "filter_assets",
            [],
            None,
        )
        self.algo.process(
            "measure_assets",
            [],
            None,
        )
        self.algo.process(
            "create_orders",
            [],
            None,
        )
