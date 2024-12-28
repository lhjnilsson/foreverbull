import time

from concurrent import futures
from multiprocessing import get_start_method
from multiprocessing import set_start_method

import grpc
import pytest

from foreverbull.algorithm import Algorithm
from foreverbull.pb.foreverbull.backtest import backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.backtest import session_service_pb2_grpc
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.session_service import SessionServiceServicer
from tests.broker import Broker


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture
def engine_stub():
    engine = Engine()
    engine.start()
    assert engine.is_ready.wait(5.0), "engine never became ready"
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    servicer = SessionServiceServicer(engine)
    engine_service_pb2_grpc.add_EngineSessionServicer_to_server(servicer, server)
    server.add_insecure_port("[::]:60066")
    server.start()
    time.sleep(1)
    yield engine_service_pb2_grpc.EngineSessionStub(grpc.insecure_channel("localhost:60066"))
    server.stop(None)
    engine.stop()
    engine.join()


@pytest.fixture
def broker_session_stub(engine_stub: engine_service_pb2_grpc.EngineSessionStub):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    bs = Broker(engine_stub)
    backtest_service_pb2_grpc.add_BacktestServicerServicer_to_server(bs, server)
    session_service_pb2_grpc.add_SessionServicerServicer_to_server(bs, server)
    server.add_insecure_port("[::]:6067")
    server.start()
    yield backtest_service_pb2_grpc.BacktestServicerStub(grpc.insecure_channel("localhost:6067"))
    server.stop(None)


@pytest.mark.parametrize("file_path", ["example_algorithms/src/example_algorithms/parallel.py"])
def test_baseline_performance(
    spawn_process,
    broker_session_stub: backtest_service_pb2_grpc.BacktestServicerStub,
    file_path,
    execution: execution_pb2.Execution,
    foreverbull_bundle,
    baseline_performance,
):
    algorithm = Algorithm.from_file_path(file_path)
    with algorithm.backtest_session("demo", broker_port=6067) as backtest:
        periods = [
            p
            for p in backtest.run_execution(
                start=execution.start_date,
                end=execution.end_date,
                symbols=[s for s in execution.symbols],
            )
        ]
        assert periods

        result, df = algorithm.get_execution("demo")

        baseline_performance = baseline_performance[df.columns]
        assert baseline_performance.equals(df), "Baseline performance does not match."
