from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2 as _execution_pb2
from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2 as _marketdata_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2 as _worker_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CreateExecutionRequest(_message.Message):
    __slots__ = ("id", "session", "start_date", "end_date", "algorithm")
    ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    id: str
    session: str
    start_date: _common_pb2.Date
    end_date: _common_pb2.Date
    algorithm: _worker_pb2.Algorithm
    def __init__(self, id: _Optional[str] = ..., session: _Optional[str] = ..., start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., algorithm: _Optional[_Union[_worker_pb2.Algorithm, _Mapping]] = ...) -> None: ...

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
    __slots__ = ("execution", "portfolio", "num_total_periods", "num_current_period")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    NUM_TOTAL_PERIODS_FIELD_NUMBER: _ClassVar[int]
    NUM_CURRENT_PERIOD_FIELD_NUMBER: _ClassVar[int]
    execution: _execution_pb2.Execution
    portfolio: _marketdata_pb2.Portfolio
    num_total_periods: int
    num_current_period: int
    def __init__(self, execution: _Optional[_Union[_execution_pb2.Execution, _Mapping]] = ..., portfolio: _Optional[_Union[_marketdata_pb2.Portfolio, _Mapping]] = ..., num_total_periods: _Optional[int] = ..., num_current_period: _Optional[int] = ...) -> None: ...

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
