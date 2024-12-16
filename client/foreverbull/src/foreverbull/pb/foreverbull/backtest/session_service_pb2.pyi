from foreverbull.pb.foreverbull.backtest import backtest_pb2 as _backtest_pb2
from foreverbull.pb.foreverbull.service import worker_pb2 as _worker_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as _finance_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2 as _execution_pb2
from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CreateExecutionRequest(_message.Message):
    __slots__ = ("backtest", "algorithm")
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    algorithm: _worker_pb2.Algorithm
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ..., algorithm: _Optional[_Union[_worker_pb2.Algorithm, _Mapping]] = ...) -> None: ...

class CreateExecutionResponse(_message.Message):
    __slots__ = ("execution", "configuration")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    execution: _execution_pb2.Execution
    configuration: _worker_pb2.ExecutionConfiguration
    def __init__(self, execution: _Optional[_Union[_execution_pb2.Execution, _Mapping]] = ..., configuration: _Optional[_Union[_worker_pb2.ExecutionConfiguration, _Mapping]] = ...) -> None: ...

class RunExecutionRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class RunExecutionResponse(_message.Message):
    __slots__ = ("execution", "portfolio")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    execution: _execution_pb2.Execution
    portfolio: _finance_pb2.Portfolio
    def __init__(self, execution: _Optional[_Union[_execution_pb2.Execution, _Mapping]] = ..., portfolio: _Optional[_Union[_finance_pb2.Portfolio, _Mapping]] = ...) -> None: ...

class StoreExecutionResultRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class StoreExecutionResultResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopServerRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopServerResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
