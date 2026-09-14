from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2 as _marketdata_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Strategy(_message.Message):
    __slots__ = ("id", "name", "description", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., created_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Version(_message.Message):
    __slots__ = ("id", "strategy_id", "strategy_name", "rev", "service_image", "parameters", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_ID_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    REV_FIELD_NUMBER: _ClassVar[int]
    SERVICE_IMAGE_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    strategy_id: str
    strategy_name: str
    rev: int
    service_image: str
    parameters: _struct_pb2.Struct
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., strategy_id: _Optional[str] = ..., strategy_name: _Optional[str] = ..., rev: _Optional[int] = ..., service_image: _Optional[str] = ..., parameters: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., created_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Experiment(_message.Message):
    __slots__ = ("id", "strategy_id", "strategy_name", "version_id", "version_rev", "backtest_name", "execution_id", "status", "error", "created_at", "completed_at")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        QUEUED: _ClassVar[Experiment.Status]
        RUNNING: _ClassVar[Experiment.Status]
        COMPLETED: _ClassVar[Experiment.Status]
        FAILED: _ClassVar[Experiment.Status]
    QUEUED: Experiment.Status
    RUNNING: Experiment.Status
    COMPLETED: Experiment.Status
    FAILED: Experiment.Status
    ID_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_ID_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_REV_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_NAME_FIELD_NUMBER: _ClassVar[int]
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    strategy_id: str
    strategy_name: str
    version_id: str
    version_rev: int
    backtest_name: str
    execution_id: str
    status: Experiment.Status
    error: str
    created_at: _timestamp_pb2.Timestamp
    completed_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., strategy_id: _Optional[str] = ..., strategy_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version_rev: _Optional[int] = ..., backtest_name: _Optional[str] = ..., execution_id: _Optional[str] = ..., status: _Optional[_Union[Experiment.Status, str]] = ..., error: _Optional[str] = ..., created_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., completed_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Deployment(_message.Message):
    __slots__ = ("id", "strategy_id", "strategy_name", "version_id", "version_rev", "mode", "backend", "calendar_code", "symbols", "schedule_kind", "dry_run", "status", "error", "last_session_date", "started_at", "stopped_at")
    class Mode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        PAPER: _ClassVar[Deployment.Mode]
        LIVE: _ClassVar[Deployment.Mode]
    PAPER: Deployment.Mode
    LIVE: Deployment.Mode
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STOPPED: _ClassVar[Deployment.Status]
        RUNNING: _ClassVar[Deployment.Status]
        ERROR: _ClassVar[Deployment.Status]
    STOPPED: Deployment.Status
    RUNNING: Deployment.Status
    ERROR: Deployment.Status
    class ScheduleKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        SESSION_CLOSE: _ClassVar[Deployment.ScheduleKind]
    SESSION_CLOSE: Deployment.ScheduleKind
    ID_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_ID_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_REV_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    BACKEND_FIELD_NUMBER: _ClassVar[int]
    CALENDAR_CODE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_KIND_FIELD_NUMBER: _ClassVar[int]
    DRY_RUN_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    LAST_SESSION_DATE_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    STOPPED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    strategy_id: str
    strategy_name: str
    version_id: str
    version_rev: int
    mode: Deployment.Mode
    backend: str
    calendar_code: str
    symbols: _containers.RepeatedScalarFieldContainer[str]
    schedule_kind: Deployment.ScheduleKind
    dry_run: bool
    status: Deployment.Status
    error: str
    last_session_date: _common_pb2.Date
    started_at: _timestamp_pb2.Timestamp
    stopped_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., strategy_id: _Optional[str] = ..., strategy_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version_rev: _Optional[int] = ..., mode: _Optional[_Union[Deployment.Mode, str]] = ..., backend: _Optional[str] = ..., calendar_code: _Optional[str] = ..., symbols: _Optional[_Iterable[str]] = ..., schedule_kind: _Optional[_Union[Deployment.ScheduleKind, str]] = ..., dry_run: bool = ..., status: _Optional[_Union[Deployment.Status, str]] = ..., error: _Optional[str] = ..., last_session_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., started_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., stopped_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class DeploymentRun(_message.Message):
    __slots__ = ("id", "deployment_id", "session_date", "started_at", "orders", "skipped", "error")
    ID_FIELD_NUMBER: _ClassVar[int]
    DEPLOYMENT_ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_DATE_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    id: str
    deployment_id: str
    session_date: _common_pb2.Date
    started_at: _timestamp_pb2.Timestamp
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    skipped: bool
    error: str
    def __init__(self, id: _Optional[str] = ..., deployment_id: _Optional[str] = ..., session_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., started_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ..., skipped: bool = ..., error: _Optional[str] = ...) -> None: ...
