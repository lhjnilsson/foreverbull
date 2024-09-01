from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from foreverbull.pb.finance import finance_pb2 as _finance_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import (
    ClassVar as _ClassVar,
    Iterable as _Iterable,
    Mapping as _Mapping,
    Optional as _Optional,
    Union as _Union,
)

DESCRIPTOR: _descriptor.FileDescriptor

class NamespaceRequestType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    GET: _ClassVar[NamespaceRequestType]
    SET: _ClassVar[NamespaceRequestType]

GET: NamespaceRequestType
SET: NamespaceRequestType

class ServiceInfoResponse(_message.Message):
    __slots__ = ("serviceType", "version", "algorithm")
    SERVICETYPE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    serviceType: str
    version: str
    algorithm: Algorithm
    def __init__(
        self,
        serviceType: _Optional[str] = ...,
        version: _Optional[str] = ...,
        algorithm: _Optional[_Union[Algorithm, _Mapping]] = ...,
    ) -> None: ...

class Request(_message.Message):
    __slots__ = ("task", "data")
    TASK_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    task: str
    data: bytes
    def __init__(self, task: _Optional[str] = ..., data: _Optional[bytes] = ...) -> None: ...

class Response(_message.Message):
    __slots__ = ("task", "data", "error")
    TASK_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    task: str
    data: bytes
    error: str
    def __init__(
        self, task: _Optional[str] = ..., data: _Optional[bytes] = ..., error: _Optional[str] = ...
    ) -> None: ...

class Algorithm(_message.Message):
    __slots__ = ("file_path", "functions", "namespaces")

    class FunctionParameter(_message.Message):
        __slots__ = ("key", "defaultValue", "value", "valueType")
        KEY_FIELD_NUMBER: _ClassVar[int]
        DEFAULTVALUE_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        VALUETYPE_FIELD_NUMBER: _ClassVar[int]
        key: str
        defaultValue: str
        value: str
        valueType: str
        def __init__(
            self,
            key: _Optional[str] = ...,
            defaultValue: _Optional[str] = ...,
            value: _Optional[str] = ...,
            valueType: _Optional[str] = ...,
        ) -> None: ...

    class Function(_message.Message):
        __slots__ = ("name", "parameters", "parallelExecution", "runFirst", "runLast")
        NAME_FIELD_NUMBER: _ClassVar[int]
        PARAMETERS_FIELD_NUMBER: _ClassVar[int]
        PARALLELEXECUTION_FIELD_NUMBER: _ClassVar[int]
        RUNFIRST_FIELD_NUMBER: _ClassVar[int]
        RUNLAST_FIELD_NUMBER: _ClassVar[int]
        name: str
        parameters: _containers.RepeatedCompositeFieldContainer[Algorithm.FunctionParameter]
        parallelExecution: bool
        runFirst: bool
        runLast: bool
        def __init__(
            self,
            name: _Optional[str] = ...,
            parameters: _Optional[_Iterable[_Union[Algorithm.FunctionParameter, _Mapping]]] = ...,
            parallelExecution: bool = ...,
            runFirst: bool = ...,
            runLast: bool = ...,
        ) -> None: ...

    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    FUNCTIONS_FIELD_NUMBER: _ClassVar[int]
    NAMESPACES_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    functions: _containers.RepeatedCompositeFieldContainer[Algorithm.Function]
    namespaces: _containers.RepeatedScalarFieldContainer[str]
    def __init__(
        self,
        file_path: _Optional[str] = ...,
        functions: _Optional[_Iterable[_Union[Algorithm.Function, _Mapping]]] = ...,
        namespaces: _Optional[_Iterable[str]] = ...,
    ) -> None: ...

class ConfigureExecutionRequest(_message.Message):
    __slots__ = ("brokerPort", "namespacePort", "databaseURL", "functions")

    class FunctionParameter(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

    class Function(_message.Message):
        __slots__ = ("name", "parameters")
        NAME_FIELD_NUMBER: _ClassVar[int]
        PARAMETERS_FIELD_NUMBER: _ClassVar[int]
        name: str
        parameters: _containers.RepeatedCompositeFieldContainer[ConfigureExecutionRequest.FunctionParameter]
        def __init__(
            self,
            name: _Optional[str] = ...,
            parameters: _Optional[_Iterable[_Union[ConfigureExecutionRequest.FunctionParameter, _Mapping]]] = ...,
        ) -> None: ...

    BROKERPORT_FIELD_NUMBER: _ClassVar[int]
    NAMESPACEPORT_FIELD_NUMBER: _ClassVar[int]
    DATABASEURL_FIELD_NUMBER: _ClassVar[int]
    FUNCTIONS_FIELD_NUMBER: _ClassVar[int]
    brokerPort: int
    namespacePort: int
    databaseURL: str
    functions: _containers.RepeatedCompositeFieldContainer[ConfigureExecutionRequest.Function]
    def __init__(
        self,
        brokerPort: _Optional[int] = ...,
        namespacePort: _Optional[int] = ...,
        databaseURL: _Optional[str] = ...,
        functions: _Optional[_Iterable[_Union[ConfigureExecutionRequest.Function, _Mapping]]] = ...,
    ) -> None: ...

class ConfigureExecutionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class WorkerRequest(_message.Message):
    __slots__ = ("task", "timestamp", "symbols", "portfolio")
    TASK_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    task: str
    timestamp: _timestamp_pb2.Timestamp
    symbols: _containers.RepeatedScalarFieldContainer[str]
    portfolio: _finance_pb2.Portfolio
    def __init__(
        self,
        task: _Optional[str] = ...,
        timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        symbols: _Optional[_Iterable[str]] = ...,
        portfolio: _Optional[_Union[_finance_pb2.Portfolio, _Mapping]] = ...,
    ) -> None: ...

class WorkerResponse(_message.Message):
    __slots__ = ("task", "orders", "error")
    TASK_FIELD_NUMBER: _ClassVar[int]
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    task: str
    orders: _containers.RepeatedCompositeFieldContainer[_finance_pb2.Order]
    error: str
    def __init__(
        self,
        task: _Optional[str] = ...,
        orders: _Optional[_Iterable[_Union[_finance_pb2.Order, _Mapping]]] = ...,
        error: _Optional[str] = ...,
    ) -> None: ...

class NamespaceRequest(_message.Message):
    __slots__ = ("key", "type", "value")
    KEY_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    key: str
    type: NamespaceRequestType
    value: _struct_pb2.Struct
    def __init__(
        self,
        key: _Optional[str] = ...,
        type: _Optional[_Union[NamespaceRequestType, str]] = ...,
        value: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...,
    ) -> None: ...

class NamespaceResponse(_message.Message):
    __slots__ = ("value", "error")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    value: _struct_pb2.Struct
    error: str
    def __init__(
        self, value: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., error: _Optional[str] = ...
    ) -> None: ...
