import tempfile

import pytest
from foreverbull import entity
from foreverbull.models import Algorithm


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
            namespaces=[],
        )

    def test_configure(self, algo):
        self._algo.configure("handle_data", "low", "5")
        self._algo.configure("handle_data", "high", "10")


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
            namespaces=[],
        )

    def test_configure(self, algo):
        self._algo.configure("handle_data", "low", "5")
        self._algo.configure("handle_data", "high", "10")


class TestWithNamespace:
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
            namespaces=["qualified_symbols", "rsi"],
        )

    def test_configure(self, algo):
        self._algo.configure("handle_data", "low", "5")
        self._algo.configure("handle_data", "high", "10")


class TestMultiStepWithNamespace:
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
            namespaces=["qualified_symbols", "asset_metrics"],
        )

    def test_configure(self, algo):
        self._algo.configure("measure_assets", "low", "5")
        self._algo.configure("measure_assets", "high", "10")
