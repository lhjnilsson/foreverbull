import time
from concurrent import futures
from datetime import datetime, timezone
from unittest.mock import MagicMock

import grpc
import pytest
from foreverbull import entity
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, broker_pb2, broker_pb2_grpc


class TestAlgorithm:
    @pytest.fixture
    def start_grpc_server(self):
        grpc_server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))

        def _add_servicer(broker_servicer, broker_session_servicer):
            broker_pb2_grpc.add_BrokerServicer_to_server(broker_servicer, grpc_server)
            broker_pb2_grpc.add_BrokerSessionServicer_to_server(broker_session_servicer, grpc_server)
            server_port = grpc_server.add_insecure_port("[::]:7877")
            grpc_server.start()
            time.sleep(1)
            return server_port

        yield _add_servicer
        grpc_server.stop(None)

    def test_get_default_no_session(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        with pytest.raises(RuntimeError, match="No backtest session"):
            algorithm.get_default()

    def test_get_default(self, parallel_algo_file, start_grpc_server):
        start = datetime.now(tz=timezone.utc)
        end = datetime.now(tz=timezone.utc)
        mocked_servicer = MagicMock(spec=broker_pb2_grpc.BrokerServicer)
        mocked_sesion_servicer = MagicMock(spec=broker_pb2_grpc.BrokerSessionServicer)
        mocked_servicer.GetBacktest.return_value = broker_pb2.GetBacktestResponse(
            name="test",
            backtest=backtest_pb2.Backtest(
                start_date=pb_utils.to_proto_timestamp(start),
                end_date=pb_utils.to_proto_timestamp(end),
                benchmark="SPY",
                symbols=["AAPL", "MSFT"],
            ),
        )
        mocked_servicer.CreateSession.return_value = broker_pb2.CreateSessionResponse(
            session=backtest_pb2.Session(
                port=None,
            )
        )
        mocked_servicer.GetSession.return_value = broker_pb2.GetSessionResponse(
            session=backtest_pb2.Session(
                port=5050,
            )
        )
        algorithm, _, _ = parallel_algo_file
        port = start_grpc_server(mocked_servicer, mocked_sesion_servicer)
        with algorithm.backtest_session("test", broker_port=port) as algo:
            assert algo.get_default() == entity.backtest.Backtest(
                name="test",
                start=start,
                end=end,
                benchmark="SPY",
                symbols=["AAPL", "MSFT"],
            )

    def test_run_execution_no_session(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        with pytest.raises(RuntimeError, match="No backtest session"):
            algorithm.run_execution(datetime.now(), datetime.now(), [])

    def test_run_execution(self, parallel_algo_file, namespace_server, start_grpc_server):
        mock_server = MagicMock(spec=broker_pb2_grpc.BrokerServicer)
        mocked_sesion_servicer = MagicMock(spec=broker_pb2_grpc.BrokerSessionServicer)
        algorithm, configuration, _ = parallel_algo_file

        mock_server.CreateSession.return_value = broker_pb2.CreateSessionResponse(
            session=backtest_pb2.Session(
                port=None,
            )
        )
        mock_server.GetSession.return_value = broker_pb2.GetSessionResponse(
            session=backtest_pb2.Session(
                port=7877,
            )
        )
        mocked_sesion_servicer.CreateExecution.return_value = broker_pb2.CreateExecutionResponse(
            configuration=configuration,
        )

        def runner(req, ctx):
            for _ in range(10):
                yield broker_pb2.RunExecutionResponse()

        mocked_sesion_servicer.RunExecution = runner
        port = start_grpc_server(mock_server, mocked_sesion_servicer)

        with algorithm.backtest_session("test", broker_port=port) as algo:
            periods = algo.run_execution(datetime.now(), datetime.now(), [])
            assert periods is not None
            assert len(list(periods)) == 10