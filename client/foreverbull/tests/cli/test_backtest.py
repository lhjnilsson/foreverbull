import traceback
from datetime import datetime
from unittest.mock import patch

import pytest
from foreverbull.cli.backtest import backtest
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.pb_utils import to_proto_timestamp
from typer.testing import CliRunner

runner = CliRunner(mix_stderr=False)


def test_backtest_list():
    with patch("foreverbull.broker.backtest.list") as mock_list:
        mock_list.return_value = [
            backtest_pb2.Backtest(
                name="test_name",
                start_date=to_proto_timestamp(datetime.now()),
                end_date=to_proto_timestamp(datetime.now()),
                symbols=["AAPL", "MSFT"],
                statuses=[
                    backtest_pb2.Backtest.Status(
                        status=backtest_pb2.Backtest.Status.Status.READY,
                        error=None,
                        occurred_at=to_proto_timestamp(datetime.now()),
                    )
                ],
            )
        ]
        result = runner.invoke(backtest, ["list"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_create():
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=to_proto_timestamp(datetime.now()),
            end_date=to_proto_timestamp(datetime.now()),
            symbols=["AAPL", "MSFT"],
            statuses=[
                backtest_pb2.Backtest.Status(
                    status=backtest_pb2.Backtest.Status.Status.CREATED,
                    error=None,
                    occurred_at=to_proto_timestamp(datetime.now()),
                )
            ],
        )
        result = runner.invoke(
            backtest,
            [
                "create",
                "test_name",
                "--start",
                "2021-01-01",
                "--end",
                "2021-01-02",
                "--symbols",
                "AAPL",
            ],
        )

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_get():
    with (patch("foreverbull.broker.backtest.get") as mock_get,):
        mock_get.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=to_proto_timestamp(datetime.now()),
            end_date=to_proto_timestamp(datetime.now()),
            symbols=["AAPL", "MSFT"],
            statuses=[
                backtest_pb2.Backtest.Status(
                    status=backtest_pb2.Backtest.Status.Status.READY,
                    error=None,
                    occurred_at=to_proto_timestamp(datetime.now()),
                )
            ],
        )
        result = runner.invoke(backtest, ["get", "test"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test" in result.stdout
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout


@pytest.mark.skip(reason="TODO")
def test_backtest_run(spawn_process, parallel_algo_file):
    algofile, _, _ = parallel_algo_file
    statuses = [
        entity.backtest.SessionStatus(
            status=entity.backtest.SessionStatusType.COMPLETED,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.backtest.SessionStatus(
            status=entity.backtest.SessionStatusType.RUNNING,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.backtest.SessionStatus(
            status=entity.backtest.SessionStatusType.CREATED,
            error=None,
            occurred_at=datetime.now(),
        ),
    ]
    with (
        patch("foreverbull.broker.backtest.run") as mock_run,
        patch("foreverbull.broker.backtest.get_session") as mock_get,
        patch("foreverbull.foreverbull.Session.new_backtest_execution") as mock_new_exc,
        patch("foreverbull.foreverbull.Session.run_backtest_execution") as mock_run_exc,
    ):
        mock_run.return_value = entity.backtest.Session(
            id="id123",
            backtest="test",
            executions=1,
            statuses=statuses[2:],
        )
        mock_get.side_effect = [
            entity.backtest.Session(
                id="id123",
                backtest="test",
                port=1234,
                executions=1,
                statuses=statuses[2:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                port=1234,
                executions=1,
                statuses=statuses[1:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                port=1234,
                executions=1,
                statuses=statuses,
            ),
        ]
        mock_new_exc.return_value = None
        mock_run_exc.return_value = None

        result = runner.invoke(backtest, ["run", algofile, "--backtest-name", "test"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "id123" in result.stdout
        assert "COMPLETED" in result.stdout
        assert "1" in result.stdout


@pytest.mark.skip(reason="TODO")
def test_backtest_run_failed(spawn_process, parallel_algo_file):
    algofile, _, _ = parallel_algo_file

    statuses = [
        entity.backtest.SessionStatus(
            status=entity.backtest.SessionStatusType.FAILED,
            error="test error",
            occurred_at=datetime.now(),
        ),
        entity.backtest.SessionStatus(
            status=entity.backtest.SessionStatusType.RUNNING,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.backtest.SessionStatus(
            status=entity.backtest.SessionStatusType.CREATED,
            error=None,
            occurred_at=datetime.now(),
        ),
    ]
    with (
        patch("foreverbull.broker.backtest.run") as mock_run,
        patch("foreverbull.broker.backtest.get_session") as mock_get,
        patch("foreverbull.foreverbull.Session.new_backtest_execution") as mock_new_exc,
        patch("foreverbull.foreverbull.Session.run_backtest_execution") as mock_run_exc,
    ):
        mock_run.return_value = entity.backtest.Session(
            id="id123",
            backtest="test",
            executions=1,
            statuses=statuses[2:],
        )
        mock_get.side_effect = [
            entity.backtest.Session(
                id="id123",
                backtest="test",
                port=1234,
                executions=1,
                statuses=statuses[2:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                port=1234,
                executions=1,
                statuses=statuses[1:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                port=1234,
                executions=1,
                statuses=statuses,
            ),
        ]
        mock_new_exc.return_value = None
        mock_run_exc.return_value = None

        result = runner.invoke(backtest, ["run", algofile, "--backtest-name", "test"])

        if not result.exit_code == 1 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "Error while running session: test error" in result.stderr
