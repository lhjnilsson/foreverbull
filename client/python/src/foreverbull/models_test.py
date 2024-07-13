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
from foreverbull import Algorithm, Function, Assets, Portfolio, Order

def handle_data(low: int, high: int, assets: Assets, portfolio: Portfolio) -> list[Order]:
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
            self._algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield self.algo

    def test_entity(self, algo):
        assert self._algo.get_entity() == entity.service.Service.Algorithm(
            file_path=self.file_path,
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
        )

    def test_configure(self, algo):
        parameters = {
            "handle_data": entity.service.Instance.Parameter(
                parameters={
                    "low": "5",
                    "high": "10",
                },
            )
        }

        self._algo.configure(parameters)


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
            self._algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(self, algo):
        assert self._algo.get_entity() == entity.service.Service.Algorithm(
            file_path=self.file_path,
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
        )

    def test_configure(self, algo):
        parameters = {
            "handle_data": entity.service.Instance.Parameter(
                parameters={
                    "low": "5",
                    "high": "10",
                },
            )
        }
        self._algo.configure(parameters)


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
    def algo(self):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self._algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(self, algo):
        assert self._algo.get_entity() == entity.service.Service.Algorithm(
            file_path=self.file_path,
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
            namespace={
                "qualified_symbols": entity.service.Service.Algorithm.Namespace(
                    type="array",
                    value_type="string",
                ),
                "rsi": entity.service.Service.Algorithm.Namespace(
                    type="object",
                    value_type="float",
                ),
            },
        )

    def test_configure(self, algo):
        parameters = {
            "handle_data": entity.service.Instance.Parameter(
                parameters={
                    "low": "5",
                    "high": "10",
                },
            )
        }
        self._algo.configure(parameters)


class TestMultiStepWithNamespace:
    example = b"""
from foreverbull import Algorithm, Function, Asset, Assets, Portfolio, Order, Namespace


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
    namespace={"qualified_symbols": list[str], "asset_metrics": dict[str, float]}
)
"""

    @pytest.fixture
    def algo(self):
        with tempfile.NamedTemporaryFile(suffix=".py") as f:
            f.write(self.example)
            f.flush()
            self._algo = Algorithm.from_file_path(f.name)
            self.file_path = f.name
            yield

    def test_entity(self, algo):
        assert self._algo.get_entity() == entity.service.Service.Algorithm(
            file_path=self.file_path,
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
            namespace={
                "qualified_symbols": entity.service.Service.Algorithm.Namespace(
                    type="array",
                    value_type="string",
                ),
                "asset_metrics": entity.service.Service.Algorithm.Namespace(
                    type="object",
                    value_type="float",
                ),
            },
        )

    def test_configure(self, algo):
        configuration = {
            "filter_assets": entity.service.Instance.Parameter(
                parameters={},
            ),
            "measure_assets": entity.service.Instance.Parameter(
                parameters={
                    "low": "5",
                    "high": "10",
                },
            ),
            "create_orders": entity.service.Instance.Parameter(
                parameters={},
            ),
        }

        self._algo.configure(configuration)
