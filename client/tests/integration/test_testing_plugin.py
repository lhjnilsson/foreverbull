import pytest

from example_algorithms import multistep_with_namespace
from example_algorithms import non_parallel
from example_algorithms import parallel
from foreverbull import Portfolio
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_testing.data import Asset
from foreverbull_testing.data import Assets


class TestParallel:
    @pytest.fixture
    def asset(self) -> Asset:
        return Asset("2020-01-01", "2020-12-31", "AAPL")

    def test_handle_data(self, asset: Asset, fb_database):
        db, _ = fb_database
        with db.connect() as conn:
            portfolio = Portfolio(
                pb=finance_pb2.Portfolio(
                    cash=1000,
                    portfolio_value=1000,
                    positions=[],
                ),
                db=conn,
            )
            with asset.with_end_date("2020-01-10") as a:
                parallel.handle_data(a, portfolio)

    """ Demo to implement later.
    def test_algo(self, asset, fb_environment):
        with fb_environment(algo, asset) as env:
            orders = env.run()
    """


class TestNonParallel:
    @pytest.fixture
    def assets(self) -> Assets:
        return Assets("2020-01-01", "2020-12-31", ["AAPL"])

    def test_handle_data(self, assets: Assets, fb_database):
        db, _ = fb_database
        with db.connect() as conn:
            portfolio = Portfolio(
                pb=finance_pb2.Portfolio(
                    cash=1000,
                    portfolio_value=1000,
                    positions=[],
                ),
                db=conn,
            )
            with assets.with_end_date("2020-01-10") as a:
                non_parallel.handle_data(a, portfolio)


class TestMultistepNamespace:
    @pytest.fixture
    def assets(self) -> Assets:
        return Assets("2020-01-01", "2020-12-31", ["AAPL"])

    @pytest.fixture
    def asset(self) -> Asset:
        return Asset("2020-01-01", "2020-12-31", "AAPL")

    def test_measure_assets(self, asset: Asset, fb_database):
        db, _ = fb_database
        with db.connect() as conn:
            portfolio = Portfolio(
                pb=finance_pb2.Portfolio(
                    cash=1000,
                    portfolio_value=1000,
                    positions=[],
                ),
                db=conn,
            )
            with asset.with_end_date("2020-01-10") as a:
                multistep_with_namespace.measure_assets(a, portfolio)

    def test_create_orders(self, assets: Assets, fb_database):
        db, _ = fb_database
        with db.connect() as conn:
            portfolio = Portfolio(
                pb=finance_pb2.Portfolio(
                    cash=1000,
                    portfolio_value=1000,
                    positions=[],
                ),
                db=conn,
            )
            with assets.with_end_date("2020-01-10") as a:
                multistep_with_namespace.create_orders(a, portfolio)
