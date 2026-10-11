from collections.abc import Callable
from collections.abc import Generator
from functools import wraps
from typing import Concatenate

import grpc

from foreverbull.broker.config import resolve_broker_address
from foreverbull.pb.foreverbull.strategy.service.v1 import strategy_service_pb2
from foreverbull.pb.foreverbull.strategy.service.v1 import strategy_service_pb2_grpc
from foreverbull.pb.foreverbull.strategy.v1 import strategy_pb2


def strategy_servicer[R, **P](
    f: Callable[Concatenate[strategy_service_pb2_grpc.StrategyServicerStub, P], R],
) -> Callable[P, R]:
    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        servicer = strategy_service_pb2_grpc.StrategyServicerStub(grpc.insecure_channel(resolve_broker_address()))
        return f(servicer, *args, **kwargs)

    return wrapper


@strategy_servicer
def create_strategy(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    name: str,
    description: str | None = None,
) -> strategy_pb2.Strategy:
    req = strategy_service_pb2.CreateStrategyRequest(name=name)
    if description:
        req.description = description
    return servicer.CreateStrategy(req).strategy


@strategy_servicer
def get_strategy(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    name: str,
) -> strategy_pb2.Strategy:
    req = strategy_service_pb2.GetStrategyRequest(name=name)
    return servicer.GetStrategy(req).strategy


@strategy_servicer
def create_version(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    strategy_name: str,
    service_image: str,
    parameters: dict | None = None,
) -> strategy_pb2.Version:
    req = strategy_service_pb2.CreateVersionRequest(
        strategy_name=strategy_name,
        service_image=service_image,
    )
    if parameters:
        req.parameters.update(parameters)
    return servicer.CreateVersion(req).version


@strategy_servicer
def create_experiment(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    strategy_name: str,
    backtest_name: str,
    version_id: str | None = None,
    version_rev: int | None = None,
) -> Generator[strategy_service_pb2.CreateExperimentResponse, None, None]:
    req = strategy_service_pb2.CreateExperimentRequest(
        strategy_name=strategy_name,
        backtest_name=backtest_name,
    )
    if version_id:
        req.version_id = version_id
    if version_rev:
        req.version_rev = version_rev
    return servicer.CreateExperiment(req)


@strategy_servicer
def list_experiments(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    strategy_name: str,
    version_id: str | None = None,
    backtest_name: str | None = None,
) -> list[strategy_pb2.Experiment]:
    req = strategy_service_pb2.ListExperimentsRequest(strategy_name=strategy_name)
    if version_id:
        req.version_id = version_id
    if backtest_name:
        req.backtest_name = backtest_name
    return servicer.ListExperiments(req).experiments


@strategy_servicer
def deploy(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    strategy_name: str,
    version_id: str | None = None,
    version_rev: int | None = None,
    dry_run: bool = True,
    confirm: bool = False,
    backend: str | None = None,
    calendar_code: str | None = None,
    symbols: list[str] | None = None,
) -> strategy_pb2.Deployment:
    req = strategy_service_pb2.DeployRequest(
        strategy_name=strategy_name,
        mode=strategy_pb2.Deployment.PAPER,
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
    return servicer.Deploy(req).deployment


@strategy_servicer
def stop_deployment(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    strategy_name: str,
    deployment_id: str | None = None,
) -> strategy_pb2.Deployment:
    req = strategy_service_pb2.StopDeploymentRequest(strategy_name=strategy_name)
    if deployment_id:
        req.deployment_id = deployment_id
    return servicer.StopDeployment(req).deployment


@strategy_servicer
def list_deployment_runs(
    servicer: strategy_service_pb2_grpc.StrategyServicerStub,
    strategy_name: str,
    deployment_id: str | None = None,
) -> list[strategy_pb2.DeploymentRun]:
    req = strategy_service_pb2.ListDeploymentRunsRequest(strategy_name=strategy_name)
    if deployment_id:
        req.deployment_id = deployment_id
    return servicer.ListDeploymentRuns(req).runs
