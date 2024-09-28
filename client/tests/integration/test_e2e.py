from concurrent import futures

import grpc
import pytest
from foreverbull.algorithm import Algorithm
from foreverbull.pb.foreverbull.backtest import (
    backtest_service_pb2_grpc,
    engine_service_pb2,
    engine_service_pb2_grpc,
    session_service_pb2_grpc,
)
from foreverbull_zipline import grpc_servicer
from foreverbull_zipline.engine import EngineProcess
from tests.broker import Broker


@pytest.fixture
def engine_stub():
    ep = EngineProcess()
    ep.start()
    if not ep.is_ready.wait(3.0):
        raise Exception("Engine not ready")
    with grpc_servicer.grpc_server(ep, port=6066):
        yield engine_service_pb2_grpc.EngineStub(
            grpc.insecure_channel("localhost:6066")
        )
    ep.stop()
    ep.join(3.0)


@pytest.fixture
def broker_session_stub(engine_stub):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    bs = Broker(engine_stub)
    backtest_service_pb2_grpc.add_BacktestServicerServicer_to_server(bs, server)
    session_service_pb2_grpc.add_SessionServicerServicer_to_server(bs, server)
    server.add_insecure_port("[::]:6067")
    server.start()
    yield backtest_service_pb2_grpc.BacktestServicerStub(
        grpc.insecure_channel("localhost:6067")
    )
    server.stop(None)


@pytest.mark.parametrize("file_path", ["tests/example_algorithms/parallel.py"])
def test_new(
    broker_session_stub: backtest_service_pb2_grpc.BacktestServicerStub,
    file_path,
    execution,
    foreverbull_bundle,
    baseline_performance,
):
    algorithm = Algorithm.from_file_path(file_path)
    with algorithm.backtest_session("demo", broker_port=6067) as backtest:
        periods = [
            p
            for p in backtest.run_execution(
                start=execution.start,
                end=execution.end,
                symbols=execution.symbols,
            )
        ]
        assert periods

        result, df = algorithm.get_execution("demo")

        baseline_performance = baseline_performance[df.columns]
        assert baseline_performance.equals(df)
