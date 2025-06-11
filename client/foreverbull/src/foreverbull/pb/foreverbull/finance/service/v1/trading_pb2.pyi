from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2 as _marketdata_pb2
from foreverbull.pb.foreverbull.finance.v1 import trading_pb2 as _trading_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2 as _worker_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetPortfolioRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetPortfolioResponse(_message.Message):
    __slots__ = ("portfolio",)
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    portfolio: _marketdata_pb2.Portfolio
    def __init__(self, portfolio: _Optional[_Union[_marketdata_pb2.Portfolio, _Mapping]] = ...) -> None: ...

class GetOrdersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetOrdersResponse(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ...) -> None: ...

class CreateSessionRequest(_message.Message):
    __slots__ = ("symbols",)
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    symbols: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, symbols: _Optional[_Iterable[str]] = ...) -> None: ...

class CreateSessionResponse(_message.Message):
    __slots__ = ("session", "status")
    class Step(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        CREATING: _ClassVar[CreateSessionResponse.Step]
        UPDATING_MARKET_DATA: _ClassVar[CreateSessionResponse.Step]
        STARTING: _ClassVar[CreateSessionResponse.Step]
        FINISHING: _ClassVar[CreateSessionResponse.Step]
    CREATING: CreateSessionResponse.Step
    UPDATING_MARKET_DATA: CreateSessionResponse.Step
    STARTING: CreateSessionResponse.Step
    FINISHING: CreateSessionResponse.Step
    class Status(_message.Message):
        __slots__ = ("step", "state")
        STEP_FIELD_NUMBER: _ClassVar[int]
        STATE_FIELD_NUMBER: _ClassVar[int]
        step: CreateSessionResponse.Step
        state: _common_pb2.State
        def __init__(self, step: _Optional[_Union[CreateSessionResponse.Step, str]] = ..., state: _Optional[_Union[_common_pb2.State, str]] = ...) -> None: ...
    SESSION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    session: _trading_pb2.Session
    status: CreateSessionResponse.Status
    def __init__(self, session: _Optional[_Union[_trading_pb2.Session, _Mapping]] = ..., status: _Optional[_Union[CreateSessionResponse.Status, _Mapping]] = ...) -> None: ...

class ConfigureRequest(_message.Message):
    __slots__ = ("algorithm",)
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    algorithm: _worker_pb2.Algorithm
    def __init__(self, algorithm: _Optional[_Union[_worker_pb2.Algorithm, _Mapping]] = ...) -> None: ...

class ConfigureResponse(_message.Message):
    __slots__ = ("configuration",)
    CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    configuration: _worker_pb2.ExecutionConfiguration
    def __init__(self, configuration: _Optional[_Union[_worker_pb2.ExecutionConfiguration, _Mapping]] = ...) -> None: ...

class RunRequest(_message.Message):
    __slots__ = ("symbols",)
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    symbols: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, symbols: _Optional[_Iterable[str]] = ...) -> None: ...

class RunResponse(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ...) -> None: ...

class GetPendingOrdersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetPendingOrdersResponse(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ...) -> None: ...

class PlaceOrderRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PlaceOrderResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopServerRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopServerResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
