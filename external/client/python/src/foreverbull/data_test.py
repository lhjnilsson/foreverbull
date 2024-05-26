import os
from datetime import datetime
from threading import Thread

import pandas
import pynng
import pytest

from foreverbull import socket
from foreverbull.data import Asset, Assets


@pytest.fixture
def namespace_server():
    namespace = dict()

    s = pynng.Rep0(listen="tcp://0.0.0.0:7878")
    s.recv_timeout = 500
    s.send_timeout = 500
    os.environ["NAMESPACE_PORT"] = "7878"

    def runner(s, namespace):
        while True:
            try:
                message = s.recv()
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            request = socket.Request.deserialize(message)
            if request.task.startswith("get:"):
                key = request.task[4:]
                response = socket.Response(task=request.task, data=namespace.get(key))
                s.send(response.serialize())
            elif request.task.startswith("set:"):
                key = request.task[4:]
                namespace[key] = request.data
                response = socket.Response(task=request.task)
                s.send(response.serialize())
            else:
                response = socket.Response(task=request.task, error="Invalid task")
                s.send(response.serialize())

    thread = Thread(target=runner, args=(s, namespace))
    thread.start()

    yield namespace

    s.close()
    thread.join()


def test_msft_09_jan(database):
    with database.connect() as conn:
        asset = Asset(datetime(2023, 1, 9, 21), conn, "MSFT")
        assert asset is not None
        assert asset.symbol == "MSFT"
        stock_data = asset.stock_data
        assert stock_data is not None
        assert isinstance(stock_data, pandas.DataFrame)
        assert len(stock_data) > 0
        assert "open" in stock_data.columns
        assert "high" in stock_data.columns
        assert "low" in stock_data.columns
        assert "close" in stock_data.columns
        assert "volume" in stock_data.columns
        assert stock_data["close"].tail(30).mean() == 239.83333333333334


def test_asset(database, backtest_entity):
    with database.connect() as conn:
        for symbol in backtest_entity.symbols:
            asset = Asset(backtest_entity.end, conn, symbol)
            assert asset is not None
            assert asset.symbol == symbol
            stock_data = asset.stock_data
            assert stock_data is not None
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) > 0
            assert "open" in stock_data.columns
            assert "high" in stock_data.columns
            assert "low" in stock_data.columns
            assert "close" in stock_data.columns
            assert "volume" in stock_data.columns


def test_asset_getattr_setattr(namespace_server):
    asset = Asset(datetime.now(), None, "AAPL")
    assert asset is not None
    asset.rsi = 56.4

    assert "rsi" in namespace_server
    assert namespace_server["rsi"] == {"AAPL": 56.4}

    namespace_server["pe"] = {"AAPL": 12.3}
    assert asset.pe == 12.3


def test_assets(database, backtest_entity):
    with database.connect() as conn:
        assets = Assets(datetime.now(), conn, backtest_entity.symbols)
        for asset in assets:
            assert asset is not None
            assert asset.symbol is not None
            stock_data = asset.stock_data
            assert stock_data is not None
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) > 0
            assert "open" in stock_data.columns
            assert "high" in stock_data.columns
            assert "low" in stock_data.columns
            assert "close" in stock_data.columns
            assert "volume" in stock_data.columns


def test_assets_getattr_setattr(namespace_server):
    assets = Assets(datetime.now(), None, [])
    assert assets is not None
    assets.holdings = ["AAPL", "MSFT"]

    assert "holdings" in namespace_server
    assert namespace_server["holdings"] == ["AAPL", "MSFT"]

    namespace_server["pe"] = {"AAPL": 12.3, "MSFT": 23.4}
    assert assets.pe == {"AAPL": 12.3, "MSFT": 23.4}
