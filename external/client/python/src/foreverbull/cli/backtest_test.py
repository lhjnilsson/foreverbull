import traceback
from datetime import datetime
from unittest.mock import patch

from typer.testing import CliRunner

from foreverbull import entity
from foreverbull.cli.backtest import backtest

runner = CliRunner(mix_stderr=False)


def test_backtest_list():
    with patch("foreverbull.broker.backtest.list") as mock_list:
        mock_list.return_value = [
            entity.backtest.Backtest(
                name="test_name",
                start=datetime.now(),
                end=datetime.now(),
                symbols=["AAPL", "MSFT"],
                statuses=[
                    entity.backtest.BacktestStatus(
                        status=entity.backtest.BacktestStatusType.READY,
                        error=None,
                        occurred_at=datetime.now(),
                    )
                ],
            )
        ]
        result = runner.invoke(backtest, ["list"])

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_create():
    get_statuses = [
        entity.backtest.BacktestStatus(
            status=entity.backtest.BacktestStatusType.READY,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.backtest.BacktestStatus(
            status=entity.backtest.BacktestStatusType.INGESTING,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.backtest.BacktestStatus(
            status=entity.backtest.BacktestStatusType.CREATED,
            error=None,
            occurred_at=datetime.now(),
        ),
    ]

    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        patch("foreverbull.broker.backtest.get") as mock_get,
    ):
        mock_create.return_value = entity.backtest.Backtest(
            name="test_name",
            start=datetime.now(),
            end=datetime.now(),
            symbols=["AAPL", "MSFT"],
            statuses=get_statuses[2:],
        )
        mock_get.side_effect = [
            entity.backtest.Backtest(
                name="test_name",
                start=datetime.now(),
                end=datetime.now(),
                symbols=["AAPL", "MSFT"],
                statuses=get_statuses[1:],
            ),
            entity.backtest.Backtest(
                name="test_name",
                start=datetime.now(),
                end=datetime.now(),
                symbols=["AAPL", "MSFT"],
                statuses=get_statuses[1:],
            ),
            entity.backtest.Backtest(
                name="test_name",
                start=datetime.now(),
                end=datetime.now(),
                symbols=["AAPL", "MSFT"],
                statuses=get_statuses,
            ),
        ]
        result = runner.invoke(
            backtest, ["create", "test_name", "--start", "2021-01-01", "--end", "2021-01-02", "--symbols", "AAPL"]
        )

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_create_error():
    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        patch("foreverbull.broker.backtest.get") as mock_get,
    ):
        mock_create.return_value = entity.backtest.Backtest(
            name="test_name",
            start=datetime.now(),
            end=datetime.now(),
            symbols=["AAPL", "MSFT"],
            statuses=[
                entity.backtest.BacktestStatus(
                    status=entity.backtest.BacktestStatusType.CREATED, error=None, occurred_at=datetime.now()
                )
            ],
        )
        mock_get.return_value = entity.backtest.Backtest(
            name="test_name",
            start=datetime.now(),
            end=datetime.now(),
            symbols=["AAPL", "MSFT"],
            statuses=[
                entity.backtest.BacktestStatus(
                    status=entity.backtest.BacktestStatusType.ERROR, error="test error", occurred_at=datetime.now()
                )
            ],
        )
        result = runner.invoke(
            backtest, ["create", "test_name", "--start", "2021-01-01", "--end", "2021-01-02", "--symbols", "AAPL"]
        )
        assert result.exit_code == 1
        assert "Error while creating backtest: test error" in result.stderr


def test_backtest_get():
    with (
        patch("foreverbull.broker.backtest.get") as mock_get,
        patch("foreverbull.broker.backtest.list_sessions") as mock_list_sessions,
    ):
        mock_get.return_value = entity.backtest.Backtest(
            name="test_name",
            start=datetime.now(),
            end=datetime.now(),
            symbols=["AAPL", "MSFT"],
            statuses=[
                entity.backtest.BacktestStatus(
                    status=entity.backtest.BacktestStatusType.READY,
                    error=None,
                    occurred_at=datetime.now(),
                )
            ],
        )
        mock_list_sessions.return_value = [
            entity.backtest.Session(
                id="id1",
                backtest="test",
                executions=1,
                statuses=[
                    entity.backtest.SessionStatus(
                        status=entity.backtest.SessionStatusType.COMPLETED,
                        error=None,
                        occurred_at=datetime.now(),
                    )
                ],
            ),
            entity.backtest.Session(
                id="id2",
                backtest="test",
                executions=1,
                statuses=[
                    entity.backtest.SessionStatus(
                        status=entity.backtest.SessionStatusType.FAILED,
                        error=None,
                        occurred_at=datetime.now(),
                    )
                ],
            ),
        ]
        result = runner.invoke(backtest, ["get", "test"])

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "test" in result.stdout
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_run(spawn_process, parallel_algo_file):
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
                executions=1,
                statuses=statuses[2:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                executions=1,
                statuses=statuses[1:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                executions=1,
                statuses=statuses,
            ),
        ]
        result = runner.invoke(backtest, ["run", parallel_algo_file, "--backtest-name", "test"])

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "id123" in result.stdout
        assert "COMPLETED" in result.stdout
        assert "1" in result.stdout


def test_backtest_run_failed(spawn_process, parallel_algo_file):
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
                executions=1,
                statuses=statuses[2:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                executions=1,
                statuses=statuses[1:],
            ),
            entity.backtest.Session(
                id="id123",
                backtest="test",
                executions=1,
                statuses=statuses,
            ),
        ]
        result = runner.invoke(backtest, ["run", parallel_algo_file, "--backtest-name", "test"])

        if not result.exit_code == 1:
            traceback.print_exception(*result.exc_info)
        assert "Error while running session: test error" in result.stderr


def test_backtest_executions():
    executions = [
        entity.backtest.Execution(
            id="id123",
            start=datetime.now(),
            end=datetime.now(),
            statuses=[
                entity.backtest.ExecutionStatus(
                    status=entity.backtest.ExecutionStatusType.COMPLETED,
                    error=None,
                    occurred_at=datetime.now(),
                )
            ],
        )
    ]
    with patch("foreverbull.broker.backtest.list_executions") as mock_list_executions:
        mock_list_executions.return_value = executions
        result = runner.invoke(backtest, ["executions", "1"])

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "id123" in result.stdout
        assert "COMPLETED" in result.stdout
        assert "1" in result.stdout
