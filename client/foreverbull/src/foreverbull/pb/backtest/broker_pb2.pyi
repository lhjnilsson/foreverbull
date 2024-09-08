from foreverbull.pb.backtest import backtest_pb2 as _backtest_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetBacktestRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetBacktestResponse(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class CreateSessionRequest(_message.Message):
    __slots__ = ("backtest_name",)
    BACKTEST_NAME_FIELD_NUMBER: _ClassVar[int]
    backtest_name: str
    def __init__(self, backtest_name: _Optional[str] = ...) -> None: ...

class CreateSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: _backtest_pb2.Session
    def __init__(self, session: _Optional[_Union[_backtest_pb2.Session, _Mapping]] = ...) -> None: ...

class GetSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class GetSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: _backtest_pb2.Session
    def __init__(self, session: _Optional[_Union[_backtest_pb2.Session, _Mapping]] = ...) -> None: ...

class CreateExecutionRequest(_message.Message):
    __slots__ = ("session_id", "backtest")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    backtest: _backtest_pb2.Backtest
    def __init__(
        self, session_id: _Optional[str] = ..., backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...
    ) -> None: ...

class CreateExecutionResponse(_message.Message):
    __slots__ = ("execution",)
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    execution: _backtest_pb2.Execution
    def __init__(self, execution: _Optional[_Union[_backtest_pb2.Execution, _Mapping]] = ...) -> None: ...

class RunExecutionRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class RunExecutionResponse(_message.Message):
    __slots__ = ("execution", "portfolio")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    execution: _backtest_pb2.Execution
    portfolio: _backtest_pb2.Portfolio
    def __init__(
        self,
        execution: _Optional[_Union[_backtest_pb2.Execution, _Mapping]] = ...,
        portfolio: _Optional[_Union[_backtest_pb2.Portfolio, _Mapping]] = ...,
    ) -> None: ...

class GetExecutionRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class GetExecutionResponse(_message.Message):
    __slots__ = ("execution",)
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    execution: _backtest_pb2.Execution
    def __init__(self, execution: _Optional[_Union[_backtest_pb2.Execution, _Mapping]] = ...) -> None: ...
