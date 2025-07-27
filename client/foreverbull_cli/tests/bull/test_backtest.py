import tempfile
import time
import traceback

from concurrent import futures
from datetime import date
from unittest.mock import MagicMock
from unittest.mock import patch

import grpc
import pytest

from foreverbull.foreverbull import BacktestSession
from foreverbull.foreverbull import Foreverbull
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2_grpc as backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2_grpc as session_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.bull.backtest import backtest


def test_backtest_list(runner):
    with patch("foreverbull.broker.backtest.list_backtests") as mock_list:
        mock_list.return_value = [
            backtest_pb2.Backtest(
                name="test_name",
                periods=[
                    backtest_pb2.Backtest.Period(
                        start_date=from_pydate_to_proto_date(date.today()),
                        end_date=from_pydate_to_proto_date(date.today()),
                        assets=[
                            backtest_pb2.Backtest.Period.Asset(
                                symbol="AAPL",
                                enabled=True,
                            ),
                            backtest_pb2.Backtest.Period.Asset(
                                symbol="MSFT",
                                enabled=True,
                            ),
                        ],
                    )
                ],
            )
        ]
        result = runner(backtest, ["list"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_create(runner):
    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        tempfile.NamedTemporaryFile() as cfg_file,
    ):
        mock_create.return_value = iter(
            [
                backtest_service_pb2.CreateBacktestResponse(
                    backtest=backtest_pb2.Backtest(
                        name="test_name",
                        periods=[
                            backtest_pb2.Backtest.Period(
                                start_date=from_pydate_to_proto_date(date.today()),
                                end_date=from_pydate_to_proto_date(date.today()),
                                assets=[
                                    backtest_pb2.Backtest.Period.Asset(
                                        symbol="AAPL",
                                        enabled=True,
                                    ),
                                    backtest_pb2.Backtest.Period.Asset(
                                        symbol="MSFT",
                                        enabled=True,
                                    ),
                                ],
                            )
                        ],
                    )
                )
            ]
        )
        cfg_file.write(b'{"start_date": "2021-01-01", "end_date": "2021-01-31", "symbols": ["AAPL", "MSFT"]}')
        cfg_file.flush()
        result = runner(
            backtest,
            [
                "create",
                cfg_file.name,
            ],
        )

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)  # type: ignore
        assert "test_name" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_get(runner):
    with (
        patch("foreverbull.broker.backtest.get") as mock_get,
        patch("foreverbull.broker.backtest.list_executions") as mock_list_executions,
    ):
        mock_get.return_value = backtest_pb2.Backtest(
            name="test_name",
            periods=[
                backtest_pb2.Backtest.Period(
                    start_date=from_pydate_to_proto_date(date.today()),
                    end_date=from_pydate_to_proto_date(date.today()),
                    assets=[
                        backtest_pb2.Backtest.Period.Asset(
                            symbol="AAPL",
                            enabled=True,
                        ),
                        backtest_pb2.Backtest.Period.Asset(
                            symbol="MSFT",
                            enabled=True,
                        ),
                    ],
                )
            ],
        )
        mock_list_executions.return_value = []
        result = runner(backtest, ["get", "test"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test" in result.stdout
        assert "AAPL,MSFT" in result.stdout


@pytest.fixture
def start_grpc_server():
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


def test_backtest_run(runner):
    mock_backtest_session = MagicMock(spec=BacktestSession)
    mock_backtest_session.run_execution.return_value = iter([marketdata_pb2.Portfolio()])

    mock_foreverbull = MagicMock(spec=Foreverbull)
    mock_foreverbull.backtest_session.return_value.__enter__.return_value = mock_backtest_session

    with patch("foreverbull.foreverbull.Foreverbull.backtest_session", new_callable=lambda: mock_foreverbull):
        with (
            patch("foreverbull.models.Algorithm.from_file_path") as from_file_path,
            patch("foreverbull.broker.backtest.create_session") as create_session,
            patch("foreverbull.broker.backtest.get") as get,
        ):
            from_file_path.return_value = True
            get.return_value = backtest_pb2.Backtest()
            create_session.return_value = iter(
                [backtest_service_pb2.CreateSessionResponse(session=session_pb2.Session(port=7878))]
            )
            result = runner(backtest, ["run", "test", "demo"])
            assert result.exit_code == 0
