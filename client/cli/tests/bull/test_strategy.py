import re

from datetime import UTC
from datetime import date
from datetime import datetime
from unittest.mock import patch

import grpc
import pytest

from foreverbull_cli.bull.cli import cli
from foreverbull_cli.bull.strategy import strategy
from foreverbull_cli.output import execution_link
from protobuf.wkt import Timestamp
from typer.testing import CliRunner

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.strategy.service.v1 import strategy_service_pb
from foreverbull.pb.foreverbull.strategy.v1 import strategy_pb
from foreverbull.pb.pb_utils import from_pydate_to_proto_date


pytestmark = pytest.mark.usefixtures("fb_env")


def _timestamp(when: datetime | None = None) -> Timestamp:
    dt = when or datetime(2024, 3, 4, 12, 0, 0)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=UTC)
    return Timestamp.from_datetime(dt)


def _sample_strategy(name="mean-rev") -> strategy_pb.Strategy:
    return strategy_pb.Strategy(
        id="strat-1",
        name=name,
        description="mean reversion",
        created_at=_timestamp(),
    )


def _sample_version(name="mean-rev", rev=1) -> strategy_pb.Version:
    return strategy_pb.Version(
        id="ver-1",
        strategy_id="strat-1",
        strategy_name=name,
        rev=rev,
        service_image="lhjnilsson/my-algo:local",
        created_at=_timestamp(),
    )


def _sample_stats(
    portfolio_value=1100.0,
    max_drawdown=-0.2,
    sharpe: float | None = 1.25,
    algorithm_period_return=0.05,
    benchmark_period_return: float | None = 0.03,
) -> strategy_pb.ExperimentStats:
    stats = strategy_pb.ExperimentStats(
        portfolio_value=portfolio_value,
        max_drawdown=max_drawdown,
        algorithm_period_return=algorithm_period_return,
    )
    if sharpe is not None:
        stats.sharpe = sharpe
    if benchmark_period_return is not None:
        stats.benchmark_period_return = benchmark_period_return
    return stats


def _sample_experiment(
    experiment_id="exp-1",
    backtest_name="spy",
    execution_id="exec-1",
    status=strategy_pb.Experiment.Status.COMPLETED,
    stats: strategy_pb.ExperimentStats | None = None,
) -> strategy_pb.Experiment:
    kwargs = {}
    if execution_id:
        kwargs["execution_id"] = execution_id
    if stats is not None:
        kwargs["stats"] = stats
    return strategy_pb.Experiment(
        id=experiment_id,
        strategy_id="strat-1",
        strategy_name="mean-rev",
        version_id="ver-1",
        version_rev=1,
        backtest_name=backtest_name,
        status=status,
        created_at=_timestamp(),
        **kwargs,
    )


def _sample_deployment(status=strategy_pb.Deployment.Status.RUNNING) -> strategy_pb.Deployment:
    return strategy_pb.Deployment(
        id="dep-1",
        strategy_id="strat-1",
        strategy_name="mean-rev",
        version_id="ver-1",
        version_rev=1,
        mode=strategy_pb.Deployment.Mode.PAPER,
        backend="default",
        calendar_code="XNYS",
        symbols=["SPY"],
        schedule_kind=strategy_pb.Deployment.ScheduleKind.SESSION_CLOSE,
        dry_run=True,
        status=status,
        last_session_date=from_pydate_to_proto_date(date(2024, 3, 4)),
    )


def _sample_run() -> strategy_pb.DeploymentRun:
    return strategy_pb.DeploymentRun(
        id="run-1",
        deployment_id="dep-1",
        session_date=from_pydate_to_proto_date(date(2024, 3, 4)),
        started_at=_timestamp(),
        orders=[marketdata_pb.Order(symbol="SPY", amount=1)],
        skipped=False,
    )


def _experiment_stream(entity: strategy_pb.Experiment):
    return iter(
        [
            strategy_service_pb.CreateExperimentResponse(
                QUEUED=common_pb.Step(state=common_pb.State.COMPLETED),
                RUNNING=common_pb.Step(state=common_pb.State.COMPLETED),
                COMPLETED=common_pb.Step(state=common_pb.State.COMPLETED),
                experiment=entity,
            )
        ]
    )


def test_strategy_create(runner):
    with patch("foreverbull.broker.strategy.create_strategy") as mock_create:
        mock_create.return_value = _sample_strategy()
        result = runner(strategy, ["create", "mean-rev", "--description", "mean reversion"])

    assert result.exit_code == 0
    mock_create.assert_called_once_with("mean-rev", description="mean reversion")
    assert "mean-rev" in result.stdout
    assert "strat-1" in result.stdout
    assert "mean reversion" in result.stdout


def test_strategy_create_via_root_cli(runner):
    with patch("foreverbull.broker.strategy.create_strategy") as mock_create:
        mock_create.return_value = _sample_strategy()
        result = runner(cli, ["strategy", "create", "mean-rev"])

    assert result.exit_code == 0
    mock_create.assert_called_once_with("mean-rev", description=None)
    assert "mean-rev" in result.stdout


def test_strategy_version_add(runner):
    with patch("foreverbull.broker.strategy.create_version") as mock_create:
        mock_create.return_value = _sample_version()
        result = runner(
            strategy,
            ["version", "add", "mean-rev", "--image", "lhjnilsson/my-algo:local"],
        )

    assert result.exit_code == 0
    mock_create.assert_called_once_with("mean-rev", "lhjnilsson/my-algo:local", parameters=None)
    assert "ver-1" in result.stdout
    assert "lhjnilsson/my-algo:local" in result.stdout
    assert "1" in result.stdout


def test_strategy_version_add_parameters(runner):
    with patch("foreverbull.broker.strategy.create_version") as mock_create:
        mock_create.return_value = _sample_version()
        result = runner(
            strategy,
            [
                "version",
                "add",
                "mean-rev",
                "--image",
                "lhjnilsson/my-algo:local",
                "--parameters",
                '{"lookback": 20}',
            ],
        )

    assert result.exit_code == 0
    mock_create.assert_called_once_with(
        "mean-rev",
        "lhjnilsson/my-algo:local",
        parameters={"lookback": 20},
    )


def test_strategy_version_add_invalid_parameters(runner):
    with patch("foreverbull.broker.strategy.create_version") as mock_create:
        result = runner(
            strategy,
            [
                "version",
                "add",
                "mean-rev",
                "--image",
                "lhjnilsson/my-algo:local",
                "--parameters",
                "not-json",
            ],
        )

    assert result.exit_code == 0
    assert "Invalid parameters JSON" in result.stdout
    mock_create.assert_not_called()


def test_strategy_experiment_backtest(runner):
    with patch("foreverbull.broker.strategy.create_experiment") as mock_create:
        mock_create.return_value = _experiment_stream(_sample_experiment(backtest_name="spy"))
        result = runner(strategy, ["experiment", "mean-rev", "--backtest", "spy", "--rev", "1"])

    assert result.exit_code == 0
    mock_create.assert_called_once_with(
        strategy_name="mean-rev",
        backtest_name="spy",
        version_id=None,
        version_rev=1,
    )
    assert "spy" in result.stdout
    assert "exp-1" in result.stdout
    assert "COMPLETED" in result.stdout
    assert f"Link: {execution_link('exec-1')}" in result.stdout


_NO_DATA = (
    "backtest 'empty' has no data to run. Experiments don't load data on their own, "
    "so delete it with `fbull backtest delete empty`, then create it again with `fbull backtest create`."
)


class _ExperimentFailed(grpc.RpcError):
    def __init__(self, details: str, code: grpc.StatusCode = grpc.StatusCode.INTERNAL):
        self._details = details
        self._code = code

    def details(self):
        return self._details

    def code(self):
        return self._code


def _experiment_stream_failing(message: str):
    running = strategy_service_pb.CreateExperimentResponse(
        QUEUED=common_pb.Step(state=common_pb.State.COMPLETED),
        RUNNING=common_pb.Step(state=common_pb.State.RUNNING),
    )
    yield running
    yield running
    yield strategy_service_pb.CreateExperimentResponse(
        QUEUED=common_pb.Step(state=common_pb.State.COMPLETED),
        RUNNING=common_pb.Step(state=common_pb.State.FAILED, message=message),
        experiment=_sample_experiment(
            backtest_name="empty", execution_id="", status=strategy_pb.Experiment.Status.FAILED
        ),
    )
    raise _ExperimentFailed(f"run experiment: {message}")


# The launcher's refusal is the whole error: one ✖ line and exit 1, no traceback.
def test_strategy_experiment_backtest_without_data_fails_with_one_line(monkeypatch):
    monkeypatch.setenv("TTY_COMPATIBLE", "0")
    with patch("foreverbull.broker.strategy.create_experiment") as mock_create:
        mock_create.return_value = _experiment_stream_failing(_NO_DATA)
        result = CliRunner().invoke(strategy, ["experiment", "mean-rev", "--backtest", "empty", "--rev", "1"])
    assert result.exit_code == 1
    assert isinstance(result.exception, SystemExit)
    assert result.stdout.count("✖") == 1
    assert f"✖ {_NO_DATA}" in " ".join(result.stdout.split())
    assert "run experiment:" not in result.stdout
    assert "auto-ingest" not in result.stdout
    assert "Traceback" not in result.stdout


def test_strategy_experiment_same_version_many_backtests(runner):
    with patch("foreverbull.broker.strategy.create_experiment") as mock_create:
        mock_create.side_effect = [
            _experiment_stream(
                _sample_experiment(experiment_id="exp-spy", backtest_name="spy", execution_id="exec-spy")
            ),
            _experiment_stream(
                _sample_experiment(experiment_id="exp-eu", backtest_name="eu-small", execution_id="exec-eu")
            ),
            _experiment_stream(
                _sample_experiment(
                    experiment_id="exp-cmdty",
                    backtest_name="commodities",
                    execution_id="exec-cmdty",
                )
            ),
        ]
        spy = runner(strategy, ["experiment", "mean-rev", "--backtest", "spy", "--version", "ver-1"])
        eu = runner(strategy, ["experiment", "mean-rev", "--backtest", "eu-small", "--version", "ver-1"])
        cmdty = runner(strategy, ["experiment", "mean-rev", "--backtest", "commodities", "--version", "ver-1"])

    assert spy.exit_code == eu.exit_code == cmdty.exit_code == 0
    assert mock_create.call_count == 3
    assert [call.kwargs["backtest_name"] for call in mock_create.call_args_list] == [
        "spy",
        "eu-small",
        "commodities",
    ]
    assert "spy" in spy.stdout
    assert "eu-small" in eu.stdout
    assert "commodities" in cmdty.stdout
    assert f"Link: {execution_link('exec-spy')}" in spy.stdout
    assert f"Link: {execution_link('exec-eu')}" in eu.stdout


def test_strategy_experiments(runner):
    with patch("foreverbull.broker.strategy.list_experiments") as mock_list:
        mock_list.return_value = [
            _sample_experiment(
                experiment_id="exp-spy",
                backtest_name="spy",
                execution_id="exec-spy",
                stats=_sample_stats(portfolio_value=1100.0, max_drawdown=-0.2, sharpe=1.25),
            ),
            _sample_experiment(
                experiment_id="exp-eu",
                backtest_name="eu-small",
                execution_id="exec-eu",
                stats=_sample_stats(
                    portfolio_value=980.0,
                    max_drawdown=-0.15,
                    sharpe=0.8,
                    benchmark_period_return=None,
                ),
            ),
        ]
        result = runner(strategy, ["experiments", "mean-rev"])

    assert result.exit_code == 0
    mock_list.assert_called_once_with(strategy_name="mean-rev", version_id=None, backtest_name=None)
    assert "Strategy" in result.stdout
    assert "mean-rev" in result.stdout
    assert "1 (ver-1)" in result.stdout
    assert "spy" in result.stdout
    assert "eu-small" in result.stdout
    assert "exec-spy" in result.stdout
    assert "exec-eu" in result.stdout
    assert "1.25000" in result.stdout
    assert "-0.20000" in result.stdout
    assert "1100.00000" in result.stdout
    assert "0.80000" in result.stdout
    assert "-0.15000" in result.stdout
    assert "980.00000" in result.stdout
    assert "0.03000" in result.stdout
    assert f"Link: {execution_link('exec-spy')}" in result.stdout
    assert f"Link: {execution_link('exec-eu')}" in result.stdout


def test_strategy_experiments_compare_one_version_two_backtests(runner):
    with patch("foreverbull.broker.strategy.list_experiments") as mock_list:
        mock_list.return_value = [
            _sample_experiment(
                experiment_id="exp-spy",
                backtest_name="spy",
                execution_id="exec-spy",
                stats=_sample_stats(portfolio_value=1100.0, max_drawdown=-0.2, sharpe=1.25),
            ),
            _sample_experiment(
                experiment_id="exp-eu",
                backtest_name="eu-small",
                execution_id="exec-eu",
                stats=_sample_stats(
                    portfolio_value=980.0,
                    max_drawdown=-0.15,
                    sharpe=0.8,
                    benchmark_period_return=0.01,
                ),
            ),
        ]
        result = runner(strategy, ["experiments", "mean-rev", "--version", "ver-1"])

    assert result.exit_code == 0
    mock_list.assert_called_once_with(strategy_name="mean-rev", version_id="ver-1", backtest_name=None)
    assert "spy" in result.stdout
    assert "eu-small" in result.stdout
    assert "1.25000" in result.stdout
    assert "0.80000" in result.stdout
    assert "1100.00000" in result.stdout
    assert "980.00000" in result.stdout
    assert f"Link: {execution_link('exec-spy')}" in result.stdout
    assert f"Link: {execution_link('exec-eu')}" in result.stdout


def test_strategy_experiments_filter_backtest(runner):
    with patch("foreverbull.broker.strategy.list_experiments") as mock_list:
        mock_list.return_value = [
            _sample_experiment(
                experiment_id="exp-spy",
                backtest_name="spy",
                execution_id="exec-spy",
                stats=_sample_stats(),
            ),
        ]
        result = runner(strategy, ["experiments", "mean-rev", "--backtest", "spy"])

    assert result.exit_code == 0
    mock_list.assert_called_once_with(strategy_name="mean-rev", version_id=None, backtest_name="spy")
    assert "spy" in result.stdout
    assert "eu-small" not in result.stdout


def test_strategy_experiments_empty_stats_without_execution(runner):
    with patch("foreverbull.broker.strategy.list_experiments") as mock_list:
        mock_list.return_value = [
            _sample_experiment(
                experiment_id="exp-queued",
                backtest_name="spy",
                execution_id="",
                status=strategy_pb.Experiment.Status.QUEUED,
            ),
        ]
        result = runner(strategy, ["experiments", "mean-rev"])

    assert result.exit_code == 0
    mock_list.assert_called_once_with(strategy_name="mean-rev", version_id=None, backtest_name=None)
    assert "QUEUED" in result.stdout
    assert "1.25000" not in result.stdout
    assert "Link:" not in result.stdout


def test_strategy_deploy_dry_run(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        mock_deploy.return_value = _sample_deployment()
        result = runner(
            strategy,
            [
                "deploy",
                "mean-rev",
                "--dry-run",
                "--mode",
                "paper",
                "--rev",
                "1",
                "--symbols",
                "SPY,QQQ",
                "--calendar",
                "XNYS",
            ],
        )

    assert result.exit_code == 0
    mock_deploy.assert_called_once_with(
        strategy_name="mean-rev",
        version_id=None,
        version_rev=1,
        dry_run=True,
        confirm=False,
        backend=None,
        calendar_code="XNYS",
        symbols=["SPY", "QQQ"],
    )
    assert "dep-1" in result.stdout
    assert "PAPER" in result.stdout
    assert "true" in result.stdout
    assert "RUNNING" in result.stdout
    assert "XNYS" in result.stdout


def test_strategy_deploy_refuses_live(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        result = runner(strategy, ["deploy", "mean-rev", "--mode", "live", "--dry-run"])

    assert result.exit_code == 0
    assert "live mode is not enabled" in result.stdout
    mock_deploy.assert_not_called()


def test_strategy_deploy_refuses_live_with_yes(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        result = runner(strategy, ["deploy", "mean-rev", "--mode", "live", "--no-dry-run", "--yes"])

    assert result.exit_code == 0
    assert "live mode is not enabled" in result.stdout
    mock_deploy.assert_not_called()


def test_strategy_deploy_refuses_non_dry_run_without_yes(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        result = runner(strategy, ["deploy", "mean-rev", "--no-dry-run"])

    assert result.exit_code == 0
    assert "requires --yes" in result.stdout
    mock_deploy.assert_not_called()


def test_strategy_deploy_paper_place_order_yes(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        mock_deploy.return_value = _sample_deployment()
        result = runner(
            strategy,
            ["deploy", "mean-rev", "--mode", "paper", "--no-dry-run", "--backend", "alpaca-paper", "--yes"],
        )

    assert result.exit_code == 0
    mock_deploy.assert_called_once_with(
        strategy_name="mean-rev",
        version_id=None,
        version_rev=None,
        dry_run=False,
        confirm=True,
        backend="alpaca-paper",
        calendar_code=None,
        symbols=[],
    )


def test_strategy_deploy_paper_place_order_confirm(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        mock_deploy.return_value = _sample_deployment()
        result = runner(
            strategy,
            ["deploy", "mean-rev", "--mode", "paper", "--no-dry-run", "--backend", "alpaca-paper", "--confirm"],
        )

    assert result.exit_code == 0
    mock_deploy.assert_called_once_with(
        strategy_name="mean-rev",
        version_id=None,
        version_rev=None,
        dry_run=False,
        confirm=True,
        backend="alpaca-paper",
        calendar_code=None,
        symbols=[],
    )


def test_strategy_deploy_no_dry_run_requires_backend(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        result = runner(strategy, ["deploy", "mean-rev", "--no-dry-run", "--yes"])

    assert result.exit_code == 0
    assert "--backend" in result.stdout
    mock_deploy.assert_not_called()


def test_strategy_deploy_no_dry_run_rejects_blank_backend(runner):
    with patch("foreverbull.broker.strategy.deploy") as mock_deploy:
        result = runner(strategy, ["deploy", "mean-rev", "--no-dry-run", "--backend", "  ", "--yes"])

    assert result.exit_code == 0
    assert "--backend" in result.stdout
    mock_deploy.assert_not_called()


def test_strategy_deploy_help_refuses_live(runner):
    result = runner(strategy, ["deploy", "--help"])

    assert result.exit_code == 0
    # Colored help (forced on GitHub Actions) puts ANSI codes between wrapped
    # words and inside highlighted options such as --backend.
    plain = re.sub(r"\x1b\[[0-9;]*[A-Za-z]", "", result.stdout)
    help_text = " ".join(plain.replace("│", " ").split())
    assert "Live mode is not supported" in help_text
    assert "live is not supported" in help_text
    assert "--backend" in help_text
    assert "Required with --no-dry-run" in help_text


def test_strategy_stop(runner):
    with patch("foreverbull.broker.strategy.stop_deployment") as mock_stop:
        mock_stop.return_value = _sample_deployment(status=strategy_pb.Deployment.Status.STOPPED)
        result = runner(strategy, ["stop", "mean-rev"])

    assert result.exit_code == 0
    mock_stop.assert_called_once_with("mean-rev", deployment_id=None)
    assert "dep-1" in result.stdout
    assert "STOPPED" in result.stdout


def test_strategy_status(runner):
    with (
        patch("foreverbull.broker.strategy.get_strategy") as mock_get,
        patch("foreverbull.broker.strategy.list_experiments") as mock_experiments,
        patch("foreverbull.broker.strategy.list_deployment_runs") as mock_runs,
    ):
        mock_get.return_value = _sample_strategy()
        mock_experiments.return_value = [
            _sample_experiment(backtest_name="spy"),
            _sample_experiment(experiment_id="exp-eu", backtest_name="eu-small", execution_id="exec-eu"),
        ]
        mock_runs.return_value = [_sample_run()]
        result = runner(strategy, ["status", "mean-rev"])

    assert result.exit_code == 0
    mock_get.assert_called_once_with("mean-rev")
    mock_experiments.assert_called_once_with(strategy_name="mean-rev")
    mock_runs.assert_called_once_with(strategy_name="mean-rev")
    assert "mean-rev" in result.stdout
    assert "spy" in result.stdout
    assert "eu-small" in result.stdout
    assert "run-1" in result.stdout
    assert "2024-03-04" in result.stdout
    assert f"Link: {execution_link('exec-1')}" in result.stdout
