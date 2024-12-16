from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.service import worker_pb2 as _worker_pb2
from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RunStrategyRequest(_message.Message):
    __slots__ = ("symbols", "start_date", "algorithm")
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    symbols: _containers.RepeatedScalarFieldContainer[str]
    start_date: _common_pb2.Date
    algorithm: _worker_pb2.Algorithm
    def __init__(self, symbols: _Optional[_Iterable[str]] = ..., start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., algorithm: _Optional[_Union[_worker_pb2.Algorithm, _Mapping]] = ...) -> None: ...

class RunStrategyResponse(_message.Message):
    __slots__ = ("status", "configuration")
    class Status(_message.Message):
        __slots__ = ("status", "error")
        class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
            __slots__ = ()
            UPDATING_MARKETDATA: _ClassVar[RunStrategyResponse.Status.Status]
            CREATING_WORKER_POOL: _ClassVar[RunStrategyResponse.Status.Status]
            READY: _ClassVar[RunStrategyResponse.Status.Status]
            RUNNING: _ClassVar[RunStrategyResponse.Status.Status]
            COMPLETED: _ClassVar[RunStrategyResponse.Status.Status]
            FAILED: _ClassVar[RunStrategyResponse.Status.Status]
        UPDATING_MARKETDATA: RunStrategyResponse.Status.Status
        CREATING_WORKER_POOL: RunStrategyResponse.Status.Status
        READY: RunStrategyResponse.Status.Status
        RUNNING: RunStrategyResponse.Status.Status
        COMPLETED: RunStrategyResponse.Status.Status
        FAILED: RunStrategyResponse.Status.Status
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        status: RunStrategyResponse.Status.Status
        error: str
        def __init__(self, status: _Optional[_Union[RunStrategyResponse.Status.Status, str]] = ..., error: _Optional[str] = ...) -> None: ...
    STATUS_FIELD_NUMBER: _ClassVar[int]
    CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    status: RunStrategyResponse.Status
    configuration: _worker_pb2.ExecutionConfiguration
    def __init__(self, status: _Optional[_Union[RunStrategyResponse.Status, _Mapping]] = ..., configuration: _Optional[_Union[_worker_pb2.ExecutionConfiguration, _Mapping]] = ...) -> None: ...
