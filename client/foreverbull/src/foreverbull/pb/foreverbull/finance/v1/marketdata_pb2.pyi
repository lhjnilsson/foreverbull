from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class OrderType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ORDER_TYPE_UNSPECIFIED: _ClassVar[OrderType]
    ORDER_TYPE_MARKET: _ClassVar[OrderType]
    ORDER_TYPE_LIMIT: _ClassVar[OrderType]

class TimeInForce(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TIME_IN_FORCE_UNSPECIFIED: _ClassVar[TimeInForce]
    TIME_IN_FORCE_DAY: _ClassVar[TimeInForce]

class PositionIntent(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    POSITION_INTENT_UNSPECIFIED: _ClassVar[PositionIntent]
    POSITION_INTENT_BUY_TO_OPEN: _ClassVar[PositionIntent]
    POSITION_INTENT_SELL_TO_CLOSE: _ClassVar[PositionIntent]
    POSITION_INTENT_BUY_TO_CLOSE: _ClassVar[PositionIntent]
    POSITION_INTENT_SELL_TO_OPEN: _ClassVar[PositionIntent]
ORDER_TYPE_UNSPECIFIED: OrderType
ORDER_TYPE_MARKET: OrderType
ORDER_TYPE_LIMIT: OrderType
TIME_IN_FORCE_UNSPECIFIED: TimeInForce
TIME_IN_FORCE_DAY: TimeInForce
POSITION_INTENT_UNSPECIFIED: PositionIntent
POSITION_INTENT_BUY_TO_OPEN: PositionIntent
POSITION_INTENT_SELL_TO_CLOSE: PositionIntent
POSITION_INTENT_BUY_TO_CLOSE: PositionIntent
POSITION_INTENT_SELL_TO_OPEN: PositionIntent

class Asset(_message.Message):
    __slots__ = ("symbol", "name")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    name: str
    def __init__(self, symbol: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class OHLC(_message.Message):
    __slots__ = ("symbol", "timestamp", "open", "high", "low", "close", "volume")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    OPEN_FIELD_NUMBER: _ClassVar[int]
    HIGH_FIELD_NUMBER: _ClassVar[int]
    LOW_FIELD_NUMBER: _ClassVar[int]
    CLOSE_FIELD_NUMBER: _ClassVar[int]
    VOLUME_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    timestamp: _timestamp_pb2.Timestamp
    open: float
    high: float
    low: float
    close: float
    volume: int
    def __init__(self, symbol: _Optional[str] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., open: _Optional[float] = ..., high: _Optional[float] = ..., low: _Optional[float] = ..., close: _Optional[float] = ..., volume: _Optional[int] = ...) -> None: ...

class Position(_message.Message):
    __slots__ = ("symbol", "amount", "cost_basis", "last_sale_price", "last_sale_date")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    AMOUNT_FIELD_NUMBER: _ClassVar[int]
    COST_BASIS_FIELD_NUMBER: _ClassVar[int]
    LAST_SALE_PRICE_FIELD_NUMBER: _ClassVar[int]
    LAST_SALE_DATE_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    amount: int
    cost_basis: float
    last_sale_price: float
    last_sale_date: _timestamp_pb2.Timestamp
    def __init__(self, symbol: _Optional[str] = ..., amount: _Optional[int] = ..., cost_basis: _Optional[float] = ..., last_sale_price: _Optional[float] = ..., last_sale_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Portfolio(_message.Message):
    __slots__ = ("timestamp", "cash_flow", "starting_cash", "portfolio_value", "pnl", "returns", "cash", "positions_value", "positions_exposure", "positions")
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    CASH_FLOW_FIELD_NUMBER: _ClassVar[int]
    STARTING_CASH_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_VALUE_FIELD_NUMBER: _ClassVar[int]
    PNL_FIELD_NUMBER: _ClassVar[int]
    RETURNS_FIELD_NUMBER: _ClassVar[int]
    CASH_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_VALUE_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_EXPOSURE_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_FIELD_NUMBER: _ClassVar[int]
    timestamp: _timestamp_pb2.Timestamp
    cash_flow: float
    starting_cash: float
    portfolio_value: float
    pnl: float
    returns: float
    cash: float
    positions_value: float
    positions_exposure: float
    positions: _containers.RepeatedCompositeFieldContainer[Position]
    def __init__(self, timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., cash_flow: _Optional[float] = ..., starting_cash: _Optional[float] = ..., portfolio_value: _Optional[float] = ..., pnl: _Optional[float] = ..., returns: _Optional[float] = ..., cash: _Optional[float] = ..., positions_value: _Optional[float] = ..., positions_exposure: _Optional[float] = ..., positions: _Optional[_Iterable[_Union[Position, _Mapping]]] = ...) -> None: ...

class Order(_message.Message):
    __slots__ = ("symbol", "amount", "type", "time_in_force", "intent")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    AMOUNT_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TIME_IN_FORCE_FIELD_NUMBER: _ClassVar[int]
    INTENT_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    amount: int
    type: OrderType
    time_in_force: TimeInForce
    intent: PositionIntent
    def __init__(self, symbol: _Optional[str] = ..., amount: _Optional[int] = ..., type: _Optional[_Union[OrderType, str]] = ..., time_in_force: _Optional[_Union[TimeInForce, str]] = ..., intent: _Optional[_Union[PositionIntent, str]] = ...) -> None: ...
