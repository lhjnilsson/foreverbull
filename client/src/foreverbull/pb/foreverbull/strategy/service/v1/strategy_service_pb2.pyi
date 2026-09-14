from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2 as _execution_pb2
from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.strategy.v1 import strategy_pb2 as _strategy_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CreateStrategyRequest(_message.Message):
    __slots__ = ("name", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class CreateStrategyResponse(_message.Message):
    __slots__ = ("strategy",)
    STRATEGY_FIELD_NUMBER: _ClassVar[int]
    strategy: _strategy_pb2.Strategy
    def __init__(self, strategy: _Optional[_Union[_strategy_pb2.Strategy, _Mapping]] = ...) -> None: ...

class GetStrategyRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetStrategyResponse(_message.Message):
    __slots__ = ("strategy",)
    STRATEGY_FIELD_NUMBER: _ClassVar[int]
    strategy: _strategy_pb2.Strategy
    def __init__(self, strategy: _Optional[_Union[_strategy_pb2.Strategy, _Mapping]] = ...) -> None: ...

class ListStrategiesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListStrategiesResponse(_message.Message):
    __slots__ = ("strategies",)
    STRATEGIES_FIELD_NUMBER: _ClassVar[int]
    strategies: _containers.RepeatedCompositeFieldContainer[_strategy_pb2.Strategy]
    def __init__(self, strategies: _Optional[_Iterable[_Union[_strategy_pb2.Strategy, _Mapping]]] = ...) -> None: ...

class DeleteStrategyRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteStrategyResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateVersionRequest(_message.Message):
    __slots__ = ("strategy_name", "service_image", "parameters")
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    SERVICE_IMAGE_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    strategy_name: str
    service_image: str
    parameters: _struct_pb2.Struct
    def __init__(self, strategy_name: _Optional[str] = ..., service_image: _Optional[str] = ..., parameters: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class CreateVersionResponse(_message.Message):
    __slots__ = ("version",)
    VERSION_FIELD_NUMBER: _ClassVar[int]
    version: _strategy_pb2.Version
    def __init__(self, version: _Optional[_Union[_strategy_pb2.Version, _Mapping]] = ...) -> None: ...

class CreateExperimentRequest(_message.Message):
    __slots__ = ("strategy_name", "version_id", "version_rev", "backtest_name")
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_REV_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_NAME_FIELD_NUMBER: _ClassVar[int]
    strategy_name: str
    version_id: str
    version_rev: int
    backtest_name: str
    def __init__(self, strategy_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version_rev: _Optional[int] = ..., backtest_name: _Optional[str] = ...) -> None: ...

class CreateExperimentResponse(_message.Message):
    __slots__ = ("QUEUED", "RUNNING", "COMPLETED", "experiment")
    QUEUED_FIELD_NUMBER: _ClassVar[int]
    RUNNING_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_FIELD_NUMBER: _ClassVar[int]
    EXPERIMENT_FIELD_NUMBER: _ClassVar[int]
    QUEUED: _common_pb2.Step
    RUNNING: _common_pb2.Step
    COMPLETED: _common_pb2.Step
    experiment: _strategy_pb2.Experiment
    def __init__(self, QUEUED: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., RUNNING: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., COMPLETED: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., experiment: _Optional[_Union[_strategy_pb2.Experiment, _Mapping]] = ...) -> None: ...

class GetExperimentRequest(_message.Message):
    __slots__ = ("experiment_id",)
    EXPERIMENT_ID_FIELD_NUMBER: _ClassVar[int]
    experiment_id: str
    def __init__(self, experiment_id: _Optional[str] = ...) -> None: ...

class GetExperimentResponse(_message.Message):
    __slots__ = ("experiment", "grafana_path", "last_period")
    EXPERIMENT_FIELD_NUMBER: _ClassVar[int]
    GRAFANA_PATH_FIELD_NUMBER: _ClassVar[int]
    LAST_PERIOD_FIELD_NUMBER: _ClassVar[int]
    experiment: _strategy_pb2.Experiment
    grafana_path: str
    last_period: _execution_pb2.Period
    def __init__(self, experiment: _Optional[_Union[_strategy_pb2.Experiment, _Mapping]] = ..., grafana_path: _Optional[str] = ..., last_period: _Optional[_Union[_execution_pb2.Period, _Mapping]] = ...) -> None: ...

class ListExperimentsRequest(_message.Message):
    __slots__ = ("strategy_name", "version_id", "backtest_name")
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_NAME_FIELD_NUMBER: _ClassVar[int]
    strategy_name: str
    version_id: str
    backtest_name: str
    def __init__(self, strategy_name: _Optional[str] = ..., version_id: _Optional[str] = ..., backtest_name: _Optional[str] = ...) -> None: ...

class ListExperimentsResponse(_message.Message):
    __slots__ = ("experiments",)
    EXPERIMENTS_FIELD_NUMBER: _ClassVar[int]
    experiments: _containers.RepeatedCompositeFieldContainer[_strategy_pb2.Experiment]
    def __init__(self, experiments: _Optional[_Iterable[_Union[_strategy_pb2.Experiment, _Mapping]]] = ...) -> None: ...

class DeployRequest(_message.Message):
    __slots__ = ("strategy_name", "version_id", "version_rev", "mode", "backend", "calendar_code", "symbols", "schedule_kind", "dry_run", "confirm")
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_REV_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    BACKEND_FIELD_NUMBER: _ClassVar[int]
    CALENDAR_CODE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_KIND_FIELD_NUMBER: _ClassVar[int]
    DRY_RUN_FIELD_NUMBER: _ClassVar[int]
    CONFIRM_FIELD_NUMBER: _ClassVar[int]
    strategy_name: str
    version_id: str
    version_rev: int
    mode: _strategy_pb2.Deployment.Mode
    backend: str
    calendar_code: str
    symbols: _containers.RepeatedScalarFieldContainer[str]
    schedule_kind: _strategy_pb2.Deployment.ScheduleKind
    dry_run: bool
    confirm: bool
    def __init__(self, strategy_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version_rev: _Optional[int] = ..., mode: _Optional[_Union[_strategy_pb2.Deployment.Mode, str]] = ..., backend: _Optional[str] = ..., calendar_code: _Optional[str] = ..., symbols: _Optional[_Iterable[str]] = ..., schedule_kind: _Optional[_Union[_strategy_pb2.Deployment.ScheduleKind, str]] = ..., dry_run: bool = ..., confirm: bool = ...) -> None: ...

class DeployResponse(_message.Message):
    __slots__ = ("deployment",)
    DEPLOYMENT_FIELD_NUMBER: _ClassVar[int]
    deployment: _strategy_pb2.Deployment
    def __init__(self, deployment: _Optional[_Union[_strategy_pb2.Deployment, _Mapping]] = ...) -> None: ...

class StopDeploymentRequest(_message.Message):
    __slots__ = ("strategy_name", "deployment_id")
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    DEPLOYMENT_ID_FIELD_NUMBER: _ClassVar[int]
    strategy_name: str
    deployment_id: str
    def __init__(self, strategy_name: _Optional[str] = ..., deployment_id: _Optional[str] = ...) -> None: ...

class StopDeploymentResponse(_message.Message):
    __slots__ = ("deployment",)
    DEPLOYMENT_FIELD_NUMBER: _ClassVar[int]
    deployment: _strategy_pb2.Deployment
    def __init__(self, deployment: _Optional[_Union[_strategy_pb2.Deployment, _Mapping]] = ...) -> None: ...

class ListDeploymentRunsRequest(_message.Message):
    __slots__ = ("strategy_name", "deployment_id")
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    DEPLOYMENT_ID_FIELD_NUMBER: _ClassVar[int]
    strategy_name: str
    deployment_id: str
    def __init__(self, strategy_name: _Optional[str] = ..., deployment_id: _Optional[str] = ...) -> None: ...

class ListDeploymentRunsResponse(_message.Message):
    __slots__ = ("runs",)
    RUNS_FIELD_NUMBER: _ClassVar[int]
    runs: _containers.RepeatedCompositeFieldContainer[_strategy_pb2.DeploymentRun]
    def __init__(self, runs: _Optional[_Iterable[_Union[_strategy_pb2.DeploymentRun, _Mapping]]] = ...) -> None: ...
