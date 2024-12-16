from foreverbull.pb.foreverbull.backtest import backtest_pb2 as _backtest_pb2
from foreverbull.pb.foreverbull.backtest import session_pb2 as _session_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2 as _execution_pb2
from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListBacktestsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListBacktestsResponse(_message.Message):
    __slots__ = ("backtests",)
    BACKTESTS_FIELD_NUMBER: _ClassVar[int]
    backtests: _containers.RepeatedCompositeFieldContainer[_backtest_pb2.Backtest]
    def __init__(self, backtests: _Optional[_Iterable[_Union[_backtest_pb2.Backtest, _Mapping]]] = ...) -> None: ...

class CreateBacktestRequest(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class CreateBacktestResponse(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class GetBacktestRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetBacktestResponse(_message.Message):
    __slots__ = ("name", "backtest")
    NAME_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    name: str
    backtest: _backtest_pb2.Backtest
    def __init__(self, name: _Optional[str] = ..., backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class CreateSessionRequest(_message.Message):
    __slots__ = ("backtest_name",)
    BACKTEST_NAME_FIELD_NUMBER: _ClassVar[int]
    backtest_name: str
    def __init__(self, backtest_name: _Optional[str] = ...) -> None: ...

class CreateSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: _session_pb2.Session
    def __init__(self, session: _Optional[_Union[_session_pb2.Session, _Mapping]] = ...) -> None: ...

class GetSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class GetSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: _session_pb2.Session
    def __init__(self, session: _Optional[_Union[_session_pb2.Session, _Mapping]] = ...) -> None: ...

class ListExecutionsRequest(_message.Message):
    __slots__ = ("backtest", "session_id")
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    backtest: str
    session_id: str
    def __init__(self, backtest: _Optional[str] = ..., session_id: _Optional[str] = ...) -> None: ...

class ListExecutionsResponse(_message.Message):
    __slots__ = ("executions",)
    EXECUTIONS_FIELD_NUMBER: _ClassVar[int]
    executions: _containers.RepeatedCompositeFieldContainer[_execution_pb2.Execution]
    def __init__(self, executions: _Optional[_Iterable[_Union[_execution_pb2.Execution, _Mapping]]] = ...) -> None: ...

class GetExecutionRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class GetExecutionResponse(_message.Message):
    __slots__ = ("execution", "periods")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    PERIODS_FIELD_NUMBER: _ClassVar[int]
    execution: _execution_pb2.Execution
    periods: _containers.RepeatedCompositeFieldContainer[_execution_pb2.Period]
    def __init__(self, execution: _Optional[_Union[_execution_pb2.Execution, _Mapping]] = ..., periods: _Optional[_Iterable[_Union[_execution_pb2.Period, _Mapping]]] = ...) -> None: ...
