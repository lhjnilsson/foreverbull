import os
import tempfile
import time
import traceback

from concurrent import futures
from datetime import date
from datetime import datetime
from unittest import mock
from unittest.mock import MagicMock
from unittest.mock import patch

import grpc
import pytest
import typer

from click.testing import Result
from typer.testing import CliRunner

from foreverbull.algorithm import Algorithm
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest import session_service_pb2_grpc
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull.pb.pb_utils import to_proto_timestamp
from foreverbull_cli.bull.backtest import backtest


@pytest.fixture
def runner():
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str]) -> Result:
        result = runner.invoke(app, args)
        echo_stdout = os.environ.get("ECHO", False)
        if echo_stdout:
            print("")
            print(result.stdout)

        return result

    return invoke


def test_backtest_list(runner):
    with patch("foreverbull.broker.backtest.list_backtests") as mock_list:
        mock_list.return_value = [
            backtest_pb2.Backtest(
                name="test_name",
                start_date=from_pydate_to_proto_date(date.today()),
                end_date=from_pydate_to_proto_date(date.today()),
                symbols=["AAPL", "MSFT"],
                statuses=[
                    backtest_pb2.Backtest.Status(
                        status=backtest_pb2.BacktestStatus.READY,
                        error=None,
                        occurred_at=to_proto_timestamp(datetime.now()),
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
                backtest_pb2.Backtest(
                    name="test_name",
                    start_date=from_pydate_to_proto_date(date.today()),
                    end_date=from_pydate_to_proto_date(date.today()),
                    symbols=["AAPL", "MSFT"],
                    statuses=[
                        backtest_pb2.Backtest.Status(
                            status=backtest_pb2.BacktestStatus.CREATED,
                            error=None,
                            occurred_at=to_proto_timestamp(datetime.now()),
                        )
                    ],
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
    ):
        mock_get.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=from_pydate_to_proto_date(date.today()),
            end_date=from_pydate_to_proto_date(date.today()),
            symbols=["AAPL", "MSFT"],
            statuses=[
                backtest_pb2.Backtest.Status(
                    status=backtest_pb2.BacktestStatus.READY,
                    error=None,
                    occurred_at=to_proto_timestamp(datetime.now()),
                )
            ],
        )
        result = runner(backtest, ["get", "test"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test" in result.stdout
        assert "READY" in result.stdout
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


def test_backtest_run(runner, start_grpc_server):
    mock_algorithm = MagicMock(spec=Algorithm)
    mock_algorithm.backtest_session.return_value.__enter__.return_value = mock_algorithm
    mock_algorithm.get_default.return_value = backtest_pb2.Backtest(
        name="test",
        start_date=common_pb2.Date(year=2022, month=1, day=1),
        end_date=common_pb2.Date(year=2022, month=12, day=31),
        symbols=["AAPL", "MSFT"],
        benchmark="SPY",
    )
    mock_algorithm.run_execution.return_value = iter([finance_pb2.Portfolio()])

    with mock.patch("foreverbull.algorithm.Algorithm.from_file_path") as inn:
        inn.return_value = mock_algorithm

        result = runner(backtest, ["run", "test", "demo"])
        assert result.exit_code == 0
