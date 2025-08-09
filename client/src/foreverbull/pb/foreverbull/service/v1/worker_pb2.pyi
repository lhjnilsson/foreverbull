from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

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
        def __init__(self, key: _Optional[str] = ..., defaultValue: _Optional[str] = ..., value: _Optional[str] = ..., valueType: _Optional[str] = ...) -> None: ...
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
        def __init__(self, name: _Optional[str] = ..., parameters: _Optional[_Iterable[_Union[Algorithm.FunctionParameter, _Mapping]]] = ..., parallelExecution: bool = ..., runFirst: bool = ..., runLast: bool = ...) -> None: ...
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    FUNCTIONS_FIELD_NUMBER: _ClassVar[int]
    NAMESPACES_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    functions: _containers.RepeatedCompositeFieldContainer[Algorithm.Function]
    namespaces: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, file_path: _Optional[str] = ..., functions: _Optional[_Iterable[_Union[Algorithm.Function, _Mapping]]] = ..., namespaces: _Optional[_Iterable[str]] = ...) -> None: ...

class ExecutionConfiguration(_message.Message):
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
        parameters: _containers.RepeatedCompositeFieldContainer[ExecutionConfiguration.FunctionParameter]
        def __init__(self, name: _Optional[str] = ..., parameters: _Optional[_Iterable[_Union[ExecutionConfiguration.FunctionParameter, _Mapping]]] = ...) -> None: ...
    BROKERPORT_FIELD_NUMBER: _ClassVar[int]
    NAMESPACEPORT_FIELD_NUMBER: _ClassVar[int]
    DATABASEURL_FIELD_NUMBER: _ClassVar[int]
    FUNCTIONS_FIELD_NUMBER: _ClassVar[int]
    brokerPort: int
    namespacePort: int
    databaseURL: str
    functions: _containers.RepeatedCompositeFieldContainer[ExecutionConfiguration.Function]
    def __init__(self, brokerPort: _Optional[int] = ..., namespacePort: _Optional[int] = ..., databaseURL: _Optional[str] = ..., functions: _Optional[_Iterable[_Union[ExecutionConfiguration.Function, _Mapping]]] = ...) -> None: ...
