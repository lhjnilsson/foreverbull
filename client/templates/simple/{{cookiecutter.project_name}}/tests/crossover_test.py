from datetime import datetime
from multiprocessing import set_start_method

import numpy
import pytest

from {{cookiecutter.project_slug}}.crossover import algo
from {{cookiecutter.project_slug}}.crossover import get_means
from {{cookiecutter.project_slug}}.crossover import should_buy

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import Backtest
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position

try:
    set_start_method("spawn")
except RuntimeError:
    pass

@pytest.mark.unit
def test_get_means(asset_manager: AssetManager):
    aapl = asset_manager.get_asset(datetime(2021, 1, 1), datetime(2021, 3, 31), "AAPL")
    short_mean, long_mean = get_means(aapl)
    assert type(short_mean) is numpy.float64
    assert type(long_mean) is numpy.float64
    assert short_mean == pytest.approx(118.6, abs=0.1)
    assert long_mean == pytest.approx(120.1, abs=0.1)
    
@pytest.mark.unit
def test_should_buy(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    aapl = asset_manager.get_asset(datetime(2021, 1, 1), datetime(2021, 3, 31), "AAPL")
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    assert not should_buy(aapl, portfolio)

@pytest.mark.backtest
def test_backtest(backtest: Backtest):
    backtest.run_execution(algo, backtest.get_default())
