import os

import pandas as pd
import pytest
from zipline.data import bundles

import foreverbull_zipline
from foreverbull import entity


@pytest.fixture(scope="session")
def foreverbull_bundle(backtest_entity: entity.backtest.Backtest, database):
    def load_or_create(bundle_name):
        try:
            return bundles.load(bundle_name, os.environ, None)
        except ValueError:
            execution = foreverbull_zipline.Execution()
            execution._ingest(backtest_entity)
            return bundles.load(bundle_name, os.environ, None)

    # sanity check
    def sanity_check(bundle):
        bundle = load_or_create("foreverbull")
        for symbol in backtest_entity.symbols:
            asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
            assert asset is not None
            start_date = pd.Timestamp(backtest_entity.start).normalize().tz_localize(None)
            asset.start_date <= start_date

            end_date = pd.Timestamp(backtest_entity.end).normalize().tz_localize(None)
            asset.end_date >= end_date

    bundle = load_or_create("foreverbull")
    sanity_check(bundle)
