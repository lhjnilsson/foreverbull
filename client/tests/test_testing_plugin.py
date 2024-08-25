import pytest
from foreverbull import Portfolio
from foreverbull_testing.data import Asset, Assets
from tests.end_to_end import multistep_with_namespace, non_parallel, parallel


class TestParallel:
    @pytest.fixture
    def asset(self) -> Asset:
        return Asset("2020-01-01", "2020-12-31", "AAPL")

    def test_handle_data(self, asset: Asset):
        portfolio = Portfolio(
            cash=1000,
            value=1000,
            positions=[],
        )
        with asset.with_end_date("2020-01-10") as a:
            orders = parallel.handle_data(a, portfolio)

    """ Demo to implement later.
    def test_algo(self, asset, fb_environment):
        with fb_environment(algo, asset) as env:
            orders = env.run()
    """


class TestNonParallel:
    @pytest.fixture
    def assets(self) -> Assets:
        return Assets("2020-01-01", "2020-12-31", ["AAPL"])

    def test_handle_data(self, assets: Assets):
        portfolio = Portfolio(
            cash=1000,
            value=1000,
            positions=[],
        )
        with assets.with_end_date("2020-01-10") as a:
            orders = non_parallel.handle_data(a, portfolio)


class TestMultistepNamespace:
    @pytest.fixture
    def assets(self) -> Assets:
        return Assets("2020-01-01", "2020-12-31", ["AAPL"])

    @pytest.fixture
    def asset(self) -> Asset:
        return Asset("2020-01-01", "2020-12-31", "AAPL")

    def test_measure_assets(self, asset: Asset):
        portfolio = Portfolio(
            cash=1000,
            value=1000,
            positions=[],
        )
        with asset.with_end_date("2020-01-10") as a:
            multistep_with_namespace.measure_assets(a, portfolio)

    def test_create_orders(self, assets: Assets):
        portfolio = Portfolio(
            cash=1000,
            value=1000,
            positions=[],
        )
        with assets.with_end_date("2020-01-10") as a:
            multistep_with_namespace.create_orders(a, portfolio)
