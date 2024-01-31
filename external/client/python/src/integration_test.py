import os
import shutil
import tempfile
from datetime import datetime
from multiprocessing import get_start_method, set_start_method

import pytest
from pynng import Req0
from zipline.utils.paths import zipline_root

from foreverbull import entity, worker
from foreverbull.entity import finance
from foreverbull.entity.service import Request, Response, SocketConfig
from foreverbull.foreverbull import Foreverbull
from foreverbull_zipline import Execution


@pytest.fixture(scope="session")
def clear_zipline_data():
    if os.path.isdir(zipline_root()):
        shutil.rmtree(zipline_root(), ignore_errors=True)


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture
def algo():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
def algo(asset: Asset, portfolio: Portfolio, ema_low: int = 16, ema_high: int = 32):
    def should_hold(df: DataFrame, low, high):
        high = EMA(df.close, timeperiod=high).iloc[-1]
        low = EMA(df.close, timeperiod=low).iloc[-1]
        if numpy.isnan(high) or low < high:
            return False
        return True

    stock_data = asset.stock_data
    if should_hold(stock_data, ema_low, ema_high):
        return finance.Order(symbol=asset.symbol, amount=1)
    else:
        return finance.Order(symbol=asset.symbol, amount=-1)
        """
        )
        f.flush()
        yield f.name


def skip_test_simple_execution(spawn_process, clear_zipline_data, populate_database, add_portfolio, algo):
    # environment.import_file(algo)

    # Ingest backtest data
    netloc = os.environ.get("POSTGRES_NETLOC", "127.0.0.1")
    database_config = entity.service.Database(
        user="postgres", password="foreverbull", netloc=netloc, port=5433, dbname="postgres"
    )
    database_url = "postgresql://{}:{}@{}:{}/{}".format(
        database_config.user,
        database_config.password,
        database_config.netloc,
        database_config.port,
        database_config.dbname,
    )
    os.environ["DATABASE_URL"] = database_url

    ingest_config = entity.backtest.IngestConfig(
        name="foreverbull",
        calendar="NYSE",
        start=datetime(2021, 1, 1),
        end=datetime(2021, 12, 30),
        symbols=["AAPL", "TSLA", "MSFT", "GOOG", "AMZN", "META"],
        database=database_url,
    )
    populate_database(ingest_config)

    client = Foreverbull(local_host="127.0.0.1", local_port=6565)
    client.set_algo(algo)
    client.setup()
    client.start()
    client_socket = Req0(dial="tcp://127.0.0.1:6565")
    client_socket.sendout = 10000
    client_socket.recv_timeout = 10000

    worker_socket_config = SocketConfig(host="127.0.0.1", port=6566, listen=False)
    worker_socket = Req0(listen=f"tcp://{worker_socket_config.host}:{worker_socket_config.port}")
    worker_socket.sendout = 10000
    worker_socket.recv_timeout = 10000

    backtest = Execution("127.0.0.1", 5656)
    backtest.start()
    backtest_socket = Req0(dial="tcp://127.0.0.1:5656")
    backtest_socket.sendout = 10000
    backtest_socket.recv_timeout = 10000
    backtest_socket.send(Request(task="info").dump())
    response = Response.load(backtest_socket.recv())
    assert response.error is None

    backtest_info = response.data
    backtest_main_socket = Req0(dial=f"tcp://{backtest_info['socket']['host']}:{backtest_info['socket']['port']}")
    backtest_main_socket.sendout = 10000
    backtest_main_socket.recv_timeout = 10000

    assert backtest_info["type"] == "backtest"
    assert backtest_info["version"]

    backtest_main_socket.send(Request(task="ingest", data=ingest_config).dump())
    response = Response.load(backtest_main_socket.recv())
    assert response.error is None

    # Configure Worker
    client_socket.send(Request(task="info").dump())
    response = Response.load(client_socket.recv())
    assert response.error is None
    assert response.data["type"] == "worker"
    assert response.data["version"]
    assert response.data["parameters"]

    execution = entity.backtest.Execution(
        id="test",
        name="foreverbull",
        bundle="foreverbull",
        calendar="NYSE",
        start=datetime(2021, 1, 7, 0, 0, 0, 0),
        end=datetime(2021, 11, 30, 0, 0, 0, 0),
        timezone="UTC",
        benchmark="AAPL",
        symbols=["AAPL", "TSLA", "MSFT", "GOOG", "AMZN", "META"],
        database=database_config,
        parameters=[
            entity.service.Parameter(key="ema_low", value="8", type="int"),
            entity.service.Parameter(key="ema_high", value="16", type="int"),
        ],
        socket=worker_socket_config,
    )

    client_socket.send(Request(task="configure_execution", data=execution).dump())
    response = Response.load(client_socket.recv())
    assert response.error is None

    backtest_main_socket.send(Request(task="configure_execution", data=execution).dump())
    response = Response.load(backtest_main_socket.recv())
    assert response.error is None

    # Run Backtest
    backtest_main_socket.send(Request(task="run_execution").dump())
    response = Response.load(backtest_main_socket.recv())
    assert response.error is None

    client_socket.send(Request(task="run_execution").dump())
    response = Response.load(client_socket.recv())
    assert response.error is None

    while True:
        backtest_main_socket.send(Request(task="get_period").dump())
        response = Response.load(backtest_main_socket.recv())
        if response.data is None:
            break
        assert response.error is None
        period = entity.backtest.Period(**response.data)
        add_portfolio("exc_123", period.timestamp, period.portfolio.cash, period.portfolio.value)
        for symbol in period.symbols:
            work = worker.Request(execution="exc_123", timestamp=period.timestamp, symbol=symbol)
            worker_socket.send(Request(task="", data=work).dump())
            response = Response.load(worker_socket.recv())
            assert response.error is None
            if response.data:
                order = finance.Order(**response.data)
                backtest_main_socket.send(Request(task="order", data=order).dump())
                response = Response.load(backtest_main_socket.recv())
                assert response.error is None
        backtest_main_socket.send(Request(task="continue").dump())
        response = Response.load(backtest_main_socket.recv())
        assert response.error is None

    # Get Results
    backtest_main_socket.send(Request(task="get_execution_result").dump())
    response = Response.load(backtest_main_socket.recv())
    assert response.error is None
    assert response.data

    result = entity.backtest.Result(**response.data)
    assert result.periods[-1].capital_used

    # Stop
    client_socket.send(Request(task="stop_execution").dump())
    response = Response.load(client_socket.recv())
    assert response.error is None

    client_socket.send(Request(task="stop").dump())
    response = Response.load(client_socket.recv())
    assert response.error is None
    client.join()
    client_socket.close()
    worker_socket.close()

    backtest_socket.send(Request(task="stop").dump())
    response = Response.load(backtest_socket.recv())
    assert response.error is None
    backtest_socket.close()
    backtest_main_socket.close()
    backtest.join()
