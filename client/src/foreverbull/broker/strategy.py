from collections.abc import Callable
from collections.abc import Generator
from functools import wraps
from typing import Concatenate

from protobuf.wkt import Struct

from foreverbull.broker.config import resolve_broker_address
from foreverbull.broker.credentials import broker_channel
from foreverbull.pb.foreverbull.strategy.service.v1 import strategy_service_pb
from foreverbull.pb.foreverbull.strategy.service.v1 import strategy_service_pb_grpc
from foreverbull.pb.foreverbull.strategy.v1 import strategy_pb
from foreverbull.pb.pb_utils import required


def strategy_servicer[R, **P](
    f: Callable[Concatenate[strategy_service_pb_grpc.StrategyServicerClientSync, P], R],
) -> Callable[P, R]:
    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        servicer = strategy_service_pb_grpc.StrategyServicerClientSync(broker_channel(resolve_broker_address()))
        return f(servicer, *args, **kwargs)

    return wrapper


@strategy_servicer
def create_strategy(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    name: str,
    description: str | None = None,
) -> strategy_pb.Strategy:
    req = strategy_service_pb.CreateStrategyRequest(name=name)
    if description:
        req.description = description
    return required(servicer.create_strategy(req).strategy, "strategy")


@strategy_servicer
def get_strategy(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    name: str,
) -> strategy_pb.Strategy:
    req = strategy_service_pb.GetStrategyRequest(name=name)
    return required(servicer.get_strategy(req).strategy, "strategy")


@strategy_servicer
def create_version(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    strategy_name: str,
    service_image: str,
    parameters: dict | None = None,
) -> strategy_pb.Version:
    req = strategy_service_pb.CreateVersionRequest(
        strategy_name=strategy_name,
        service_image=service_image,
    )
    if parameters:
        req.parameters = Struct.from_python(parameters)
    return required(servicer.create_version(req).version, "version")


@strategy_servicer
def create_experiment(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    strategy_name: str,
    backtest_name: str,
    version_id: str | None = None,
    version_rev: int | None = None,
) -> Generator[strategy_service_pb.CreateExperimentResponse, None, None]:
    req = strategy_service_pb.CreateExperimentRequest(
        strategy_name=strategy_name,
        backtest_name=backtest_name,
    )
    if version_id:
        req.version_id = version_id
    if version_rev:
        req.version_rev = version_rev
    yield from servicer.create_experiment(req)


@strategy_servicer
def list_experiments(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    strategy_name: str,
    version_id: str | None = None,
    backtest_name: str | None = None,
) -> list[strategy_pb.Experiment]:
    req = strategy_service_pb.ListExperimentsRequest(strategy_name=strategy_name)
    if version_id:
        req.version_id = version_id
    if backtest_name:
        req.backtest_name = backtest_name
    return servicer.list_experiments(req).experiments


@strategy_servicer
def deploy(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    strategy_name: str,
    version_id: str | None = None,
    version_rev: int | None = None,
    dry_run: bool = True,
    confirm: bool = False,
    backend: str | None = None,
    calendar_code: str | None = None,
    symbols: list[str] | None = None,
) -> strategy_pb.Deployment:
    req = strategy_service_pb.DeployRequest(
        strategy_name=strategy_name,
        mode=strategy_pb.Deployment.Mode.PAPER,
        dry_run=dry_run,
        confirm=confirm,
        symbols=symbols or [],
    )
    if version_id:
        req.version_id = version_id
    if version_rev:
        req.version_rev = version_rev
    if backend:
        req.backend = backend
    if calendar_code:
        req.calendar_code = calendar_code
    return required(servicer.deploy(req).deployment, "deployment")


@strategy_servicer
def stop_deployment(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    strategy_name: str,
    deployment_id: str | None = None,
) -> strategy_pb.Deployment:
    req = strategy_service_pb.StopDeploymentRequest(strategy_name=strategy_name)
    if deployment_id:
        req.deployment_id = deployment_id
    return required(servicer.stop_deployment(req).deployment, "deployment")


@strategy_servicer
def list_deployment_runs(
    servicer: strategy_service_pb_grpc.StrategyServicerClientSync,
    strategy_name: str,
    deployment_id: str | None = None,
) -> list[strategy_pb.DeploymentRun]:
    req = strategy_service_pb.ListDeploymentRunsRequest(strategy_name=strategy_name)
    if deployment_id:
        req.deployment_id = deployment_id
    return servicer.list_deployment_runs(req).runs
