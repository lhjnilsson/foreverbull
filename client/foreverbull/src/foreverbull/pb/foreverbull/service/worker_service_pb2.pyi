from google.protobuf import struct_pb2 as _struct_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as _finance_pb2
from foreverbull.pb.foreverbull.service import worker_pb2 as _worker_pb2
from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class NamespaceRequestType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    GET: _ClassVar[NamespaceRequestType]
    SET: _ClassVar[NamespaceRequestType]
GET: NamespaceRequestType
SET: NamespaceRequestType

class GetServiceInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetServiceInfoResponse(_message.Message):
    __slots__ = ("algorithm",)
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    algorithm: _worker_pb2.Algorithm
    def __init__(self, algorithm: _Optional[_Union[_worker_pb2.Algorithm, _Mapping]] = ...) -> None: ...

class ConfigureExecutionRequest(_message.Message):
    __slots__ = ("configuration",)
    CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    configuration: _worker_pb2.ExecutionConfiguration
    def __init__(self, configuration: _Optional[_Union[_worker_pb2.ExecutionConfiguration, _Mapping]] = ...) -> None: ...

class ConfigureExecutionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RunExecutionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RunExecutionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class WorkerRequest(_message.Message):
    __slots__ = ("task", "symbols", "portfolio")
    TASK_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    task: str
    symbols: _containers.RepeatedScalarFieldContainer[str]
    portfolio: _finance_pb2.Portfolio
    def __init__(self, task: _Optional[str] = ..., symbols: _Optional[_Iterable[str]] = ..., portfolio: _Optional[_Union[_finance_pb2.Portfolio, _Mapping]] = ...) -> None: ...

class WorkerResponse(_message.Message):
    __slots__ = ("task", "orders", "error")
    TASK_FIELD_NUMBER: _ClassVar[int]
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    task: str
    orders: _containers.RepeatedCompositeFieldContainer[_finance_pb2.Order]
    error: str
    def __init__(self, task: _Optional[str] = ..., orders: _Optional[_Iterable[_Union[_finance_pb2.Order, _Mapping]]] = ..., error: _Optional[str] = ...) -> None: ...

class NamespaceRequest(_message.Message):
    __slots__ = ("key", "type", "value")
    KEY_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    key: str
    type: NamespaceRequestType
    value: _struct_pb2.Struct
    def __init__(self, key: _Optional[str] = ..., type: _Optional[_Union[NamespaceRequestType, str]] = ..., value: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class NamespaceResponse(_message.Message):
    __slots__ = ("value", "error")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    value: _struct_pb2.Struct
    error: str
    def __init__(self, value: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...
