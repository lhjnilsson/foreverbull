import time

from concurrent import futures
from contextlib import suppress
from datetime import date
from unittest.mock import MagicMock

import grpc
import pytest

from foreverbull.foreverbull import BacktestSession
from foreverbull.foreverbull import Foreverbull
from foreverbull.foreverbull import TradingSession
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2_grpc as backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2 as backtest_session_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2_grpc as backtest_session_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2 as trading_service_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2_grpc as trading_service_pb2_grpc
from foreverbull.pb.foreverbull.finance.v1 import trading_pb2


class TestBacktestSession:
    @pytest.fixture
    def start_grpc_server(self):
        grpc_server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))

        def _add_servicer(backtest_session_servicer):
            backtest_session_service_pb2_grpc.add_SessionServicerServicer_to_server(
                backtest_session_servicer, grpc_server
            )
            server_port = grpc_server.add_insecure_port("[::]:7877")
            grpc_server.start()
            time.sleep(0.5)
            stub = backtest_session_service_pb2_grpc.SessionServicerStub(
                grpc.insecure_channel(f"localhost:{server_port}")
            )
            return stub

        try:
            yield _add_servicer
        finally:
            with suppress(Exception):
                grpc_server.stop(None)

    def test_run_execution(self, parallel_algo_file, namespace_server, start_grpc_server):
        model, configuration, _ = parallel_algo_file
        backtest = backtest_pb2.Backtest()
        session = session_pb2.Session()
        mocked_session_servicer = MagicMock(spec=backtest_session_service_pb2_grpc.SessionServicerServicer)
        mocked_session_servicer.CreateExecution.return_value = backtest_session_service_pb2.CreateExecutionResponse(
            configuration=configuration,
        )
        mocked_session_servicer.StoreResult.return_value = backtest_session_service_pb2.StoreExecutionResultResponse()
        mocked_session_servicer.StopServer.return_value = backtest_session_service_pb2.StopServerResponse()

        def runner(req, ctx):
            for _ in range(10):
                yield backtest_session_service_pb2.RunExecutionResponse()

        mocked_session_servicer.RunExecution = runner

        stub = start_grpc_server(mocked_session_servicer)
        uut = BacktestSession(model, backtest, session, stub)
        periods = uut.run_execution(
            pb_utils.from_pydate_to_proto_date(date.today()),
            pb_utils.from_pydate_to_proto_date(date.today()),
        )
        assert periods is not None
        assert len(list(periods)) == 10


class TestTradingSession:
    @pytest.fixture
    def start_grpc_server(self):
        grpc_server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))

        def _add_servicer(trading_session_servicer):
            trading_service_pb2_grpc.add_SessionServicerServicer_to_server(trading_session_servicer, grpc_server)
            server_port = grpc_server.add_insecure_port("[::]:7877")
            grpc_server.start()
            time.sleep(0.5)
            stub = trading_service_pb2_grpc.SessionServicerStub(grpc.insecure_channel(f"localhost:{server_port}"))
            return stub

        try:
            yield _add_servicer
        finally:
            with suppress(Exception):
                grpc_server.stop(None)

    def test_run_trading(self, parallel_algo_file, namespace_server, start_grpc_server):
        model, configuration, _ = parallel_algo_file
        mocked_session_servicer = MagicMock(spec=trading_service_pb2_grpc.SessionServicerServicer)
        mocked_session_servicer.Configure.return_value = trading_service_pb2.ConfigureResponse(
            configuration=configuration,
        )
        mocked_session_servicer.Run.return_value = trading_service_pb2.RunResponse(
            orders=[],
        )

        stub = start_grpc_server(mocked_session_servicer)
        uut = TradingSession(model, stub)
        orders, _ = uut.run_trading([])
        assert orders == []


class TestForeverbull:
    @pytest.fixture
    def backtest_grpc_server(self):
        grpc_server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))

        def _add_servicer(backtest_servicer, backtest_session_servicer):
            backtest_service_pb2_grpc.add_BacktestServicerServicer_to_server(backtest_servicer, grpc_server)
            if backtest_session_servicer:
                backtest_session_service_pb2_grpc.add_SessionServicerServicer_to_server(
                    backtest_session_servicer, grpc_server
                )
            server_port = grpc_server.add_insecure_port("[::]:7877")
            grpc_server.start()
            time.sleep(1)
            return server_port

        try:
            yield _add_servicer
        finally:
            with suppress(Exception):
                grpc_server.stop(None)

    @pytest.fixture
    def trading_grpc_server(self):
        grpc_server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))

        def _add_servicer(trading_servicer, trading_session_servicer):
            trading_service_pb2_grpc.add_TradingServicerServicer_to_server(trading_servicer, grpc_server)
            if trading_session_servicer:
                trading_service_pb2_grpc.add_SessionServicerServicer_to_server(trading_session_servicer, grpc_server)
            server_port = grpc_server.add_insecure_port("[::]:7877")
            grpc_server.start()
            time.sleep(1)
            return server_port

        try:
            yield _add_servicer
        finally:
            with suppress(Exception):
                grpc_server.stop(None)

    def test_backtest_session(self, parallel_algo_file, backtest_grpc_server):
        mock_backtest_session_servicer = MagicMock(spec=backtest_session_service_pb2_grpc.SessionServicerServicer)
        mock_backtest_session_servicer.StopServer.return_value = backtest_session_service_pb2.StopServerResponse()
        mock_backtest_servicer = MagicMock(spec=backtest_service_pb2_grpc.BacktestServicer)
        model, _, _ = parallel_algo_file
        algorithm = Foreverbull(model)
        port = backtest_grpc_server(mock_backtest_servicer, mock_backtest_session_servicer)
        backtest = backtest_pb2.Backtest()
        backtest_session = session_pb2.Session(
            port=port,
        )

        with algorithm.backtest_session(backtest, backtest_session) as algo:
            assert type(algo) is BacktestSession, "algo is not of type BacktestSession"

    def test_trading_session(self, parallel_algo_file, trading_grpc_server):
        mock_trading_session_servicer = MagicMock(spec=trading_service_pb2_grpc.SessionServicerServicer)
        mock_trading_session_servicer.StopServer.return_value = trading_service_pb2.StopServerResponse()
        mock_trading_servicer = MagicMock(spec=trading_service_pb2_grpc.TradingServicer)
        model, _, _ = parallel_algo_file
        algorithm = Foreverbull(model)
        port = trading_grpc_server(mock_trading_servicer, mock_trading_session_servicer)
        trading_session = trading_pb2.Session(
            port=port,
        )
        with algorithm.trading_session(trading_session) as algo:
            assert type(algo) is TradingSession, "algo is not of type TradingSession"
