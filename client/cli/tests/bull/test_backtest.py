import tempfile
import time
import traceback

from concurrent import futures
from datetime import date
from datetime import timedelta
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import grpc
import pytest
import yaml

from foreverbull_cli.bull.backtest import backtest
from typer.testing import CliRunner
from foreverbull_cli.bull.cli import cli
from foreverbull_cli.output import execution_link

from foreverbull.foreverbull import BacktestSession
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2_grpc as backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2 as session_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2_grpc as session_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2
from foreverbull.pb.pb_utils import from_pydate_to_proto_date


pytestmark = pytest.mark.usefixtures("fb_env")

START = date(2020, 1, 1)
END = date(2020, 1, 31)


def _sample_backtest(name="test_name", benchmark="SPY", provider="") -> backtest_pb2.Backtest:
    return backtest_pb2.Backtest(
        name=name,
        benchmark=benchmark,
        provider=provider,
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=from_pydate_to_proto_date(START),
                end_date=from_pydate_to_proto_date(END),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="MSFT", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="IBM", enabled=False),
                ],
            )
        ],
    )


def _sample_execution(execution_id="exec-1") -> execution_pb2.Execution:
    return execution_pb2.Execution(
        id=execution_id,
        backtest="test_name",
        session="sess-1",
        start_date=from_pydate_to_proto_date(START),
        end_date=from_pydate_to_proto_date(END),
    )


def test_backtest_list(runner):
    with patch("foreverbull.broker.backtest.list_backtests") as mock_list:
        mock_list.return_value = [_sample_backtest()]
        result = runner(backtest, ["list"])

        if result.exit_code != 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "SPY" in result.stdout
        assert "2020-01-01" in result.stdout
        assert "2020-01-31" in result.stdout


def test_backtest_create(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = iter(
            [
                backtest_service_pb2.CreateBacktestResponse(
                    CREATING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    DOWNLOADING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    INGESTING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    STORING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    backtest=_sample_backtest(),
                )
            ]
        )
        result = runner(
            backtest,
            [
                "create",
            ],
            input="demo\niShares S&P 100\n2020-01-01\n2020-01-31\n",
        )

        if result.exit_code != 0:
            traceback.print_exception(*result.exc_info)  # type: ignore
        assert "test_name" in result.stdout
        assert "2020-01-01" in result.stdout
        assert "2020-01-31" in result.stdout
        assert "AAPL" in result.stdout
        assert "MSFT" in result.stdout
        assert "SPY" in result.stdout


def test_backtest_create_reprompts_invalid_dates(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = iter(
            [
                backtest_service_pb2.CreateBacktestResponse(
                    CREATING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    DOWNLOADING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    INGESTING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    STORING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    backtest=_sample_backtest(),
                )
            ]
        )
        result = runner(
            backtest,
            ["create"],
            input="\ndemo\niShares S&P 100\nnot-a-date\n1999-01-01\n2020-01-01\n2019-12-31\n2020-01-31\n",
        )

    assert result.exit_code == 0
    assert "Enter a name" in result.stdout
    assert "not a date" in result.stdout
    assert "started on 2000-10-23" in result.stdout
    assert "before start date" in result.stdout
    request = mock_create.call_args.kwargs["entity"]
    assert request.name == "demo"
    assert request.provider == "stooq"
    assert request.start_date.year == 2020
    assert request.end_date.day == 31


def test_backtest_etfs(runner):
    result = runner(backtest, ["etfs"])
    assert result.exit_code == 0
    assert "iShares S&P 100" in result.stdout
    assert "OEF" in result.stdout
    assert "2000-10-23" in result.stdout


def _created_backtest_response():
    return iter(
        [
            backtest_service_pb2.CreateBacktestResponse(
                CREATING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                DOWNLOADING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                INGESTING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                STORING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                backtest=_sample_backtest(),
            )
        ]
    )


def test_backtest_create_unknown_etf_does_not_call_broker(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        result = runner(backtest, ["create"], input="demo\nnot-a-fund\n")

    assert result.exit_code == 0
    mock_create.assert_not_called()
    assert "Unknown ETF 'not-a-fund'. Known funds: iShares S&P 100." in result.stdout
    assert "Creating" not in result.stdout
    assert "Traceback" not in result.stdout


def test_backtest_create_reprompts_unknown_etf(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = _created_backtest_response()
        result = runner(
            backtest,
            ["create"],
            input="demo\nnot-a-fund\niShares S&P 100\n2020-01-01\n2020-01-31\n",
        )

    assert result.exit_code == 0
    assert "Unknown ETF 'not-a-fund'. Known funds: iShares S&P 100." in result.stdout
    request = mock_create.call_args.kwargs["entity"]
    assert request.base_etf == "iShares S&P 100"
    assert request.name == "demo"


def test_backtest_create_reprompts_end_after_today(runner):
    future = date.today() + timedelta(days=1)
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = _created_backtest_response()
        result = runner(
            backtest,
            ["create"],
            input=f"demo\niShares S&P 100\n2020-01-01\n{future.isoformat()}\n2020-01-31\n",
        )

    assert result.exit_code == 0
    assert f"End date {future.isoformat()} is after today ({date.today().isoformat()})." in result.stdout
    request = mock_create.call_args.kwargs["entity"]
    assert request.end_date.year == 2020
    assert request.end_date.month == 1
    assert request.end_date.day == 31
    assert mock_create.call_count == 1


def test_backtest_create_reprompts_start_after_today(runner):
    future = date.today() + timedelta(days=1)
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = _created_backtest_response()
        result = runner(
            backtest,
            ["create"],
            input=f"demo\niShares S&P 100\n{future.isoformat()}\n2020-01-01\n2020-01-31\n",
        )

    assert result.exit_code == 0
    assert f"{future.isoformat()} is after today ({date.today().isoformat()})." in result.stdout
    request = mock_create.call_args.kwargs["entity"]
    assert request.start_date.year == 2020
    assert request.start_date.day == 1


def test_backtest_create_reprompts_non_iso_date(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = _created_backtest_response()
        result = runner(
            backtest,
            ["create"],
            input="demo\niShares S&P 100\n20200101\n2020-02-31\n2020-01-01\n2020-01-31\n",
        )

    assert result.exit_code == 0
    assert "'20200101' is not a date. Use YYYY-MM-DD, for example 2024-01-31." in result.stdout
    assert "'2020-02-31' is not a date. Use YYYY-MM-DD, for example 2024-01-31." in result.stdout
    request = mock_create.call_args.kwargs["entity"]
    assert request.start_date.year == 2020
    assert request.start_date.month == 1
    assert request.start_date.day == 1


def test_backtest_create_accepts_long_range(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = _created_backtest_response()
        result = runner(
            backtest,
            ["create"],
            input="long\niShares S&P 100\n2020-01-01\n2025-12-31\n",
        )

    assert result.exit_code == 0
    assert "is not a date" not in result.stdout
    assert "after today" not in result.stdout
    assert "before start date" not in result.stdout
    assert "started on" not in result.stdout
    request = mock_create.call_args.kwargs["entity"]
    assert request.name == "long"
    assert request.base_etf == "iShares S&P 100"
    assert (request.start_date.year, request.start_date.month, request.start_date.day) == (2020, 1, 1)
    assert (request.end_date.year, request.end_date.month, request.end_date.day) == (2025, 12, 31)


def test_backtest_create_from_file(runner):
    payload = {
        "name": "from_file",
        "benchmark": "QQQ",
        "periods": [
            {
                "start_date": "2021-01-01",
                "end_date": "2021-01-31",
                "assets": [
                    {"symbol": "AAPL", "enabled": True},
                    {"symbol": "MSFT", "enabled": True},
                ],
            }
        ],
    }
    created = backtest_pb2.Backtest(
        name="from_file",
        benchmark="QQQ",
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=from_pydate_to_proto_date(date(2021, 1, 1)),
                end_date=from_pydate_to_proto_date(date(2021, 1, 31)),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="MSFT", enabled=True),
                ],
            )
        ],
    )
    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as cfg_file,
    ):
        yaml.safe_dump(payload, cfg_file)
        cfg_file.flush()
        mock_create.return_value = iter(
            [
                backtest_service_pb2.CreateBacktestResponse(
                    STORING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    backtest=created,
                )
            ]
        )
        result = runner(backtest, ["create", "--file", cfg_file.name])

    Path(cfg_file.name).unlink(missing_ok=True)
    assert result.exit_code == 0
    request = mock_create.call_args.kwargs["entity"]
    assert request.backtest.provider == "stooq"
    assert request.backtest.profile == ""
    assert "from_file" in result.stdout
    assert "2021-01-01" in result.stdout
    assert "QQQ" in result.stdout
    assert "AAPL" in result.stdout
    assert "Using provider: stooq" in result.stdout
    assert "stooq" in result.stdout


def _file_payload(**extra: object) -> dict:
    payload = {
        "name": "from_file",
        "benchmark": "QQQ",
        "periods": [
            {
                "start_date": "2021-01-01",
                "end_date": "2021-01-31",
                "assets": [
                    {"symbol": "AAPL", "enabled": True},
                ],
            }
        ],
    }
    payload.update(extra)
    return payload


def test_backtest_create_from_file_reads_provider_and_profile(runner):
    created = _sample_backtest(name="from_file", benchmark="QQQ", provider="alpaca")
    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as cfg_file,
    ):
        yaml.safe_dump(_file_payload(provider="alpaca", profile="alpaca-paper"), cfg_file)
        cfg_file.flush()
        mock_create.return_value = iter(
            [
                backtest_service_pb2.CreateBacktestResponse(
                    STORING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    backtest=created,
                )
            ]
        )
        result = runner(backtest, ["create", "--file", cfg_file.name])

    Path(cfg_file.name).unlink(missing_ok=True)
    assert result.exit_code == 0
    request = mock_create.call_args.kwargs["entity"]
    assert request.backtest.provider == "alpaca"
    assert request.backtest.profile == "alpaca-paper"
    assert "alpaca" in result.stdout


def test_backtest_create_provider_flag_overrides_file(runner):
    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as cfg_file,
    ):
        yaml.safe_dump(_file_payload(provider="yahoo", profile="alpaca-paper"), cfg_file)
        cfg_file.flush()
        mock_create.return_value = iter(
            [
                backtest_service_pb2.CreateBacktestResponse(
                    STORING=common_pb2.Step(state=common_pb2.State.STATE_COMPLETED),
                    backtest=_sample_backtest(name="from_file", benchmark="QQQ", provider="stooq"),
                )
            ]
        )
        result = runner(backtest, ["create", "--file", cfg_file.name, "--provider", "stooq"])

    Path(cfg_file.name).unlink(missing_ok=True)
    assert result.exit_code == 0
    request = mock_create.call_args.kwargs["entity"]
    assert request.backtest.provider == "stooq"
    assert request.backtest.profile == "alpaca-paper"
    assert "Using provider: stooq" in result.stdout
    assert "stooq" in result.stdout


def test_backtest_create_unknown_provider_does_not_call_broker():
    # CliRunner catches typer.Exit; the shared runner fixture re-raises it.
    with patch("foreverbull.broker.backtest.create") as mock_create:
        result = CliRunner().invoke(backtest, ["create", "--provider", "binance"])

    assert result.exit_code == 1
    mock_create.assert_not_called()
    assert "Unknown price provider 'binance'" in result.stdout
    assert "Valid providers: yahoo, stooq, alpaca" in result.stdout


def test_backtest_create_etf_sends_provider_flag(runner):
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = _created_backtest_response()
        result = runner(
            backtest,
            ["create", "--provider", "stooq"],
            input="demo\niShares S&P 100\n2020-01-01\n2020-01-31\n",
        )

    assert result.exit_code == 0
    request = mock_create.call_args.kwargs["entity"]
    assert request.provider == "stooq"
    assert request.name == "demo"
    assert "Using provider: stooq" in result.stdout


def test_backtest_get(runner):
    with (
        patch("foreverbull.broker.backtest.get") as mock_get,
        patch("foreverbull.broker.backtest.list_executions") as mock_list_executions,
    ):
        mock_get.return_value = _sample_backtest()
        mock_list_executions.return_value = [_sample_execution()]
        result = runner(backtest, ["get", "test_name"])

        if result.exit_code != 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "AAPL" in result.stdout
        assert "MSFT" in result.stdout
        assert "IBM" in result.stdout
        assert "2020-01-01" in result.stdout
        assert "2020-01-31" in result.stdout
        assert "exec-1" in result.stdout
        assert f"Link: {execution_link('exec-1')}" in result.stdout
        assert "SPY" in result.stdout
        assert "stooq" in result.stdout


def test_backtest_get_yaml(runner):
    with (
        patch("foreverbull.broker.backtest.get") as mock_get,
        patch("foreverbull.broker.backtest.list_executions") as mock_list_executions,
    ):
        mock_get.return_value = _sample_backtest()
        mock_list_executions.return_value = []
        result = runner(backtest, ["get", "test_name", "--output", "yaml"])
        assert result.exit_code == 0
        dumped = yaml.safe_load(result.stdout)
        assert dumped["name"] == "test_name"
        assert dumped["benchmark"] == "SPY"
        assert dumped["provider"] == "stooq"
        assert "profile" not in dumped
        assert dumped["periods"][0]["start_date"] == "2020-01-01"
        assert dumped["periods"][0]["assets"][0]["symbol"] == "AAPL"


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


def _run_session_mocks():
    mock_backtest_session = MagicMock(spec=BacktestSession)
    mock_backtest_session.backtest = _sample_backtest()
    mock_backtest_session.run_execution.return_value = iter(
        [
            (
                "exec-1",
                session_service_pb2.RunExecutionResponse(num_current_period=1, num_total_periods=2),
            )
        ]
    )
    return mock_backtest_session


def test_backtest_run(runner):
    mock_backtest_session = _run_session_mocks()
    with (
        patch("foreverbull_cli.bull.backtest.Foreverbull") as mock_foreverbull,
        patch("foreverbull.models.Algorithm.from_file_path") as from_file_path,
        patch("foreverbull.broker.backtest.create_session") as create_session,
        patch("foreverbull.broker.backtest.get") as get,
    ):
        from_file_path.return_value = True
        get.return_value = _sample_backtest()
        create_session.return_value = iter(
            [backtest_service_pb2.CreateSessionResponse(session=session_pb2.Session(port=7878))]
        )
        mock_foreverbull.return_value.backtest_session.return_value.__enter__.return_value = mock_backtest_session
        result = runner(backtest, ["run", "test_name", "demo.py"])
        assert result.exit_code == 0
        assert "exec-1" in result.stdout
        assert "test_name" in result.stdout
        assert "2020-01-01" in result.stdout
        assert f"Link: {execution_link('exec-1')}" in result.stdout
        mock_backtest_session.run_execution.assert_called_once()
        create_session.assert_called_once()


def test_backtest_run_dry(runner):
    with (
        patch("foreverbull.broker.backtest.get") as get,
        patch("foreverbull.broker.backtest.create_session") as create_session,
    ):
        get.return_value = _sample_backtest()
        result = runner(backtest, ["run", "test_name", "demo.py", "--dry"])
        assert result.exit_code == 0
        assert "Dry Run" in result.stdout
        assert "demo.py" in result.stdout
        assert "2020-01-01" in result.stdout
        create_session.assert_not_called()


def test_backtest_run_invalid_start(runner):
    with patch("foreverbull.broker.backtest.get") as get:
        get.return_value = _sample_backtest()
        result = runner(backtest, ["run", "test_name", "demo.py", "--start", "not-a-date"])
        assert result.exit_code == 0
        assert "Invalid start date" in result.stdout


def test_backtest_executions(runner):
    with patch("foreverbull.broker.backtest.list_executions") as mock_list_executions:
        mock_list_executions.return_value = [_sample_execution()]
        result = runner(backtest, ["executions", "--backtest", "test_name"])
        assert result.exit_code == 0
        assert "exec-1" in result.stdout
        assert "test_name" in result.stdout
        assert "sess-1" in result.stdout
        assert "2020-01-01" in result.stdout
        assert f"Link: {execution_link('exec-1')}" in result.stdout


def test_backtest_execution(runner):
    period = execution_pb2.Period(
        date=from_pydate_to_proto_date(date(2020, 1, 2)),
        portfolio_value=1000.5,
        alpha=0.12,
        beta=0.98,
        sharpe=1.5,
        sortino=1.2,
        benchmark_period_return=0.01,
        benchmark_volatility=0.02,
    )
    with patch("foreverbull.broker.backtest.get_execution") as mock_get_execution:
        mock_get_execution.return_value = (_sample_execution(), [period])
        result = runner(backtest, ["execution", "exec-1"])
        assert result.exit_code == 0
        assert "exec-1" in result.stdout
        assert "1000.50000" in result.stdout
        assert "0.12000" in result.stdout
        assert "1.50000" in result.stdout
        assert "2020-01-02" in result.stdout
        assert f"Link: {execution_link('exec-1')}" in result.stdout


def test_verbosity_flags(runner):
    with (
        patch("foreverbull.broker.backtest.list_backtests", return_value=[]),
        patch("foreverbull_cli.bull.cli.logging.basicConfig") as basic_config,
    ):
        result = runner(cli, ["backtest", "list"])
        assert result.exit_code == 0
        assert basic_config.call_args.kwargs["level"] == "ERROR"

        result = runner(cli, ["-v", "backtest", "list"])
        assert result.exit_code == 0
        assert basic_config.call_args.kwargs["level"] == "WARNING"

        result = runner(cli, ["-vv", "backtest", "list"])
        assert result.exit_code == 0
        assert basic_config.call_args.kwargs["level"] == "INFO"

        result = runner(cli, ["-vvv", "backtest", "list"])
        assert result.exit_code == 0
        assert basic_config.call_args.kwargs["level"] == "DEBUG"

        result = runner(cli, ["-v", "-v", "backtest", "list"])
        assert result.exit_code == 0
        assert basic_config.call_args.kwargs["level"] == "INFO"
