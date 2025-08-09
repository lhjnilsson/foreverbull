from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2 as _marketdata_pb2
from foreverbull.pb.foreverbull.finance.v1 import trading_pb2 as _trading_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2 as _worker_pb2
from google.protobuf.internal import containers as _containers
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
    __slots__ = ("CREATING", "UPDATING_MARKET_DATA", "STARTING", "FINISHING", "session")
    CREATING_FIELD_NUMBER: _ClassVar[int]
    UPDATING_MARKET_DATA_FIELD_NUMBER: _ClassVar[int]
    STARTING_FIELD_NUMBER: _ClassVar[int]
    FINISHING_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CREATING: _common_pb2.Step
    UPDATING_MARKET_DATA: _common_pb2.Step
    STARTING: _common_pb2.Step
    FINISHING: _common_pb2.Step
    session: _trading_pb2.Session
    def __init__(self, CREATING: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., UPDATING_MARKET_DATA: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., STARTING: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., FINISHING: _Optional[_Union[_common_pb2.Step, _Mapping]] = ..., session: _Optional[_Union[_trading_pb2.Session, _Mapping]] = ...) -> None: ...

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

class PlaceOrderRequest(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ...) -> None: ...

class PlaceOrderResponse(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ...) -> None: ...

class StopServerRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopServerResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
