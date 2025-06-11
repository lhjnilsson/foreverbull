from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2 as _backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2 as _execution_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2 as _session_pb2
from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
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
    __slots__ = ("backtest", "status")
    class Step(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        CREATING: _ClassVar[CreateBacktestResponse.Step]
        DOWNLOADING: _ClassVar[CreateBacktestResponse.Step]
        INGESTING: _ClassVar[CreateBacktestResponse.Step]
        STORING: _ClassVar[CreateBacktestResponse.Step]
    CREATING: CreateBacktestResponse.Step
    DOWNLOADING: CreateBacktestResponse.Step
    INGESTING: CreateBacktestResponse.Step
    STORING: CreateBacktestResponse.Step
    class Status(_message.Message):
        __slots__ = ("step", "state")
        STEP_FIELD_NUMBER: _ClassVar[int]
        STATE_FIELD_NUMBER: _ClassVar[int]
        step: CreateBacktestResponse.Step
        state: _common_pb2.State
        def __init__(self, step: _Optional[_Union[CreateBacktestResponse.Step, str]] = ..., state: _Optional[_Union[_common_pb2.State, str]] = ...) -> None: ...
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    status: CreateBacktestResponse.Status
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ..., status: _Optional[_Union[CreateBacktestResponse.Status, _Mapping]] = ...) -> None: ...

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
    __slots__ = ("session", "status")
    class Step(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        CREATING: _ClassVar[CreateSessionResponse.Step]
        STARTING: _ClassVar[CreateSessionResponse.Step]
        STORING: _ClassVar[CreateSessionResponse.Step]
    CREATING: CreateSessionResponse.Step
    STARTING: CreateSessionResponse.Step
    STORING: CreateSessionResponse.Step
    class Status(_message.Message):
        __slots__ = ("step", "state")
        STEP_FIELD_NUMBER: _ClassVar[int]
        STATE_FIELD_NUMBER: _ClassVar[int]
        step: CreateSessionResponse.Step
        state: _common_pb2.State
        def __init__(self, step: _Optional[_Union[CreateSessionResponse.Step, str]] = ..., state: _Optional[_Union[_common_pb2.State, str]] = ...) -> None: ...
    SESSION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    session: _session_pb2.Session
    status: CreateSessionResponse.Status
    def __init__(self, session: _Optional[_Union[_session_pb2.Session, _Mapping]] = ..., status: _Optional[_Union[CreateSessionResponse.Status, _Mapping]] = ...) -> None: ...

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
