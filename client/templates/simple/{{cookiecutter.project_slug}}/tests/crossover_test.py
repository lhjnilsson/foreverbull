import numpy

from crossover import algo
from crossover import get_means
from crossover import should_buy

from foreverbull_testing.models import Backtest
from foreverbull_testing.models import Position
from foreverbull_testing.session import Session


def test_get_means(session: Session):
    times = session.cross_up("SMA", 10, 30)
    asset = next(iter(session.assets(["SMA"], times[-1])))
    short_mean, long_mean = get_means(asset)
    assert type(short_mean) is numpy.float64
    assert type(long_mean) is numpy.float64
    assert short_mean > 0
    assert long_mean > 0
    assert short_mean > long_mean


def test_should_buy(session: Session):
    times = session.cross_up("SMA", 10, 30)
    as_of = times[-1]
    asset = next(iter(session.assets(["SMA"], as_of)))
    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="SMA", amount=10)])
    assert not should_buy(asset, portfolio)


def test_backtest(backtest: Backtest):
    backtest.run_execution(algo, backtest.get_default())
