import os
from datetime import datetime
from threading import Thread

import pandas
import pynng
import pytest
from foreverbull.data import Asset, Assets
from foreverbull.pb.service import service_pb2


@pytest.fixture
def namespace_server():
    namespace = dict()

    s = pynng.Rep0(listen="tcp://0.0.0.0:7878")
    s.recv_timeout = 500
    s.send_timeout = 500
    os.environ["NAMESPACE_PORT"] = "7878"

    def runner(s, namespace):
        while True:
            request = service_pb2.NamespaceRequest()
            try:
                data = s.recv()
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            request.ParseFromString(data)
            if request.type == service_pb2.NamespaceRequestType.GET:
                response = service_pb2.NamespaceResponse()
                response.value.update(namespace.get(request.key, {}))
            elif request.type == service_pb2.NamespaceRequestType.SET:
                namespace[request.key] = {k: v for k, v in request.value.items()}
                response = service_pb2.NamespaceResponse()
                response.value.update(namespace[request.key])
            else:
                response = service_pb2.NamespaceResponse(error="Invalid request type")
            s.send(response.SerializeToString())

    thread = Thread(target=runner, args=(s, namespace))
    thread.start()

    yield namespace

    s.close()
    thread.join()


def test_asset_getattr_setattr(fb_database, namespace_server):
    database, _ = fb_database
    with database.connect() as conn:
        asset = Asset(datetime.now(), conn, "AAPL")
        assert asset is not None
        asset.rsi = 56.4

        assert "rsi" in namespace_server
        assert namespace_server["rsi"] == {"AAPL": 56.4}

        namespace_server["pe"] = {"AAPL": 12.3}
        assert asset.pe == 12.3


def test_assets(fb_database, backtest_entity):
    database, ensure_data = fb_database
    ensure_data(backtest_entity)
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


def test_assets_getattr_setattr(fb_database, namespace_server):
    database, _ = fb_database
    with database.connect() as conn:
        assets = Assets(datetime.now(), conn, [])
        assert assets is not None
        assets.holdings = {"AAPL": True, "MSFT": False}

        assert "holdings" in namespace_server
        assert namespace_server["holdings"] == {"AAPL": True, "MSFT": False}

        namespace_server["pe"] = {"AAPL": 12.3, "MSFT": 23.4}
        assert assets.pe == {"AAPL": 12.3, "MSFT": 23.4}
