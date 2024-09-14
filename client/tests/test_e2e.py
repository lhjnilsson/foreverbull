from concurrent import futures

import grpc
import pytest
from foreverbull import entity
from foreverbull.algorithm import Algorithm
from foreverbull.gprc_service import new_grpc_server
from foreverbull.pb.backtest import broker_pb2_grpc, engine_pb2_grpc
from foreverbull.pb.service import worker_pb2_grpc
from foreverbull_zipline import grpc_servicer
from foreverbull_zipline.engine import EngineProcess
from tests.broker import Broker


@pytest.fixture
def engine_stub():
    ep = EngineProcess()
    ep.start()
    if not ep.is_ready.wait(3.0):
        raise Exception("Engine not ready")
    with grpc_servicer.grpc_server(ep, port=6066) as server:
        yield engine_pb2_grpc.EngineStub(grpc.insecure_channel("localhost:6066"))
    ep.stop()
    ep.join(3.0)


@pytest.fixture
def broker_session_stub(engine_stub):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    bs = Broker(engine_stub)
    broker_pb2_grpc.add_BrokerSessionServicer_to_server(bs, server)
    broker_pb2_grpc.add_BrokerServicer_to_server(bs, server)
    server.add_insecure_port("[::]:6067")
    server.start()
    yield broker_pb2_grpc.BrokerSessionStub(grpc.insecure_channel("localhost:6067"))
    server.stop(None)


from datetime import datetime, timezone


@pytest.mark.parametrize("file_path", ["tests/example_algorithms/parallel.py"])
def test_new(
    broker_session_stub: broker_pb2_grpc.BrokerSessionStub,
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
        print("DR: ", df)

        baseline_performance = baseline_performance[df.columns]
        assert baseline_performance.equals(df)
