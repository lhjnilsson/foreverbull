import time

from concurrent import futures
from datetime import date
from unittest.mock import MagicMock

import grpc
import pytest

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import backtest_service_pb2
from foreverbull.pb.foreverbull.backtest import backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.backtest import session_pb2
from foreverbull.pb.foreverbull.backtest import session_service_pb2
from foreverbull.pb.foreverbull.backtest import session_service_pb2_grpc


class TestAlgorithm:
    @pytest.fixture
    def start_grpc_server(self):
        grpc_server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))

        def _add_servicer(broker_servicer, broker_session_servicer):
            backtest_service_pb2_grpc.add_BacktestServicerServicer_to_server(broker_servicer, grpc_server)
            session_service_pb2_grpc.add_SessionServicerServicer_to_server(broker_session_servicer, grpc_server)
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
        mocked_servicer = MagicMock(spec=backtest_service_pb2_grpc.BacktestServicer)
        mocked_sesion_servicer = MagicMock(spec=session_service_pb2_grpc.SessionServicerServicer)
        mocked_servicer.GetBacktest.return_value = backtest_service_pb2.GetBacktestResponse(
            name="test",
            backtest=backtest_pb2.Backtest(
                start_date=pb_utils.from_pydate_to_proto_date(date.today()),
                end_date=pb_utils.from_pydate_to_proto_date(date.today()),
                benchmark="SPY",
                symbols=["AAPL", "MSFT"],
            ),
        )
        mocked_servicer.CreateSession.return_value = backtest_service_pb2.CreateSessionResponse(
            session=session_pb2.Session(
                port=None,
            )
        )
        mocked_servicer.GetSession.return_value = backtest_service_pb2.GetSessionResponse(
            session=session_pb2.Session(
                port=7877,
            )
        )
        mocked_sesion_servicer.StopServer.return_value = session_service_pb2.StopServerResponse()
        algorithm, _, _ = parallel_algo_file
        port = start_grpc_server(mocked_servicer, mocked_sesion_servicer)
        with algorithm.backtest_session("test", broker_port=port) as algo:
            assert algo

    def test_run_execution_no_session(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        with pytest.raises(RuntimeError, match="No backtest session"):
            for _ in algorithm.run_execution(
                pb_utils.from_pydate_to_proto_date(date.today()),
                pb_utils.from_pydate_to_proto_date(date.today()),
                [],
            ):
                pass

    def test_run_execution(self, parallel_algo_file, namespace_server, start_grpc_server):
        mock_server = MagicMock(spec=backtest_service_pb2_grpc.BacktestServicer)
        mocked_sesion_servicer = MagicMock(spec=session_service_pb2_grpc.SessionServicerServicer)
        algorithm, configuration, _ = parallel_algo_file

        mock_server.CreateSession.return_value = backtest_service_pb2.CreateSessionResponse(
            session=session_pb2.Session(
                port=None,
            )
        )
        mock_server.GetSession.return_value = backtest_service_pb2.GetSessionResponse(
            session=session_pb2.Session(
                port=7877,
            )
        )
        mocked_sesion_servicer.CreateExecution.return_value = session_service_pb2.CreateExecutionResponse(
            configuration=configuration,
        )
        mocked_sesion_servicer.StoreResult.return_value = session_service_pb2.StoreExecutionResultResponse()
        mocked_sesion_servicer.StopServer.return_value = session_service_pb2.StopServerResponse()

        def runner(req, ctx):
            for _ in range(10):
                yield session_service_pb2.RunExecutionResponse()

        mocked_sesion_servicer.RunExecution = runner
        port = start_grpc_server(mock_server, mocked_sesion_servicer)

        with algorithm.backtest_session("test", broker_port=port) as algo:
            periods = algo.run_execution(
                pb_utils.from_pydate_to_proto_date(date.today()),
                pb_utils.from_pydate_to_proto_date(date.today()),
                [],
            )
            assert periods is not None
            assert len(list(periods)) == 10

    def test_get_execution_no_session(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        with pytest.raises(RuntimeError, match="No backtest session"):
            algorithm.get_execution("test")

    def test_get_execution(self, parallel_algo_file, start_grpc_server):
        mock_server = MagicMock(spec=backtest_service_pb2_grpc.BacktestServicer)
        algorithm, _, _ = parallel_algo_file
        mock_server.CreateSession.return_value = backtest_service_pb2.CreateSessionResponse(
            session=session_pb2.Session(
                port=None,
            )
        )
        mock_server.GetSession.return_value = backtest_service_pb2.GetSessionResponse(
            session=session_pb2.Session(
                port=7877,
            )
        )
        mocked_sesion_servicer = MagicMock(spec=session_service_pb2_grpc.SessionServicerServicer)
        mocked_sesion_servicer.StopServer.return_value = session_service_pb2.StopServerResponse()
        mock_server.GetExecution.return_value = backtest_service_pb2.GetExecutionResponse(
            execution=execution_pb2.Execution(),
            periods=[
                execution_pb2.Period(
                    portfolio_value=1000,
                    returns=0.1,
                    alpha=0.1,
                    beta=0.1,
                    sharpe=0.1,
                )
            ],
        )
        port = start_grpc_server(mock_server, mocked_sesion_servicer)
        with algorithm.backtest_session("test", broker_port=port) as algo:
            execution, periods = algo.get_execution("test")
            assert execution is not None
            assert periods is not None
            assert len(periods) == 1
            assert periods["portfolio_value"].iloc[0] == 1000
            assert periods["returns"].iloc[0] == 0.1
            assert periods["alpha"].iloc[0] == 0.1
            assert periods["beta"].iloc[0] == 0.1
            assert periods["sharpe"].iloc[0] == 0.1
