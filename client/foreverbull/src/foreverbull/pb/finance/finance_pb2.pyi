from google.protobuf import timestamp_pb2 as _timestamp_pb2
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

class OrderStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    OPEN: _ClassVar[OrderStatus]
    FILLED: _ClassVar[OrderStatus]
    CANCELED: _ClassVar[OrderStatus]
    REJECTED: _ClassVar[OrderStatus]
    HELD: _ClassVar[OrderStatus]

OPEN: OrderStatus
FILLED: OrderStatus
CANCELED: OrderStatus
REJECTED: OrderStatus
HELD: OrderStatus

class Position(_message.Message):
    __slots__ = ("symbol", "exchange", "amount", "cost")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    EXCHANGE_FIELD_NUMBER: _ClassVar[int]
    AMOUNT_FIELD_NUMBER: _ClassVar[int]
    COST_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    exchange: str
    amount: float
    cost: float
    def __init__(
        self,
        symbol: _Optional[str] = ...,
        exchange: _Optional[str] = ...,
        amount: _Optional[float] = ...,
        cost: _Optional[float] = ...,
    ) -> None: ...

class Portfolio(_message.Message):
    __slots__ = ("cash", "value", "positions")
    CASH_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_FIELD_NUMBER: _ClassVar[int]
    cash: float
    value: float
    positions: _containers.RepeatedCompositeFieldContainer[Position]
    def __init__(
        self,
        cash: _Optional[float] = ...,
        value: _Optional[float] = ...,
        positions: _Optional[_Iterable[_Union[Position, _Mapping]]] = ...,
    ) -> None: ...

class Order(_message.Message):
    __slots__ = ("id", "symbol", "amount", "filled", "commission", "limit_price", "stop_price", "created_at", "status")
    ID_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    AMOUNT_FIELD_NUMBER: _ClassVar[int]
    FILLED_FIELD_NUMBER: _ClassVar[int]
    COMMISSION_FIELD_NUMBER: _ClassVar[int]
    LIMIT_PRICE_FIELD_NUMBER: _ClassVar[int]
    STOP_PRICE_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    id: str
    symbol: str
    amount: int
    filled: int
    commission: float
    limit_price: float
    stop_price: float
    created_at: _timestamp_pb2.Timestamp
    status: OrderStatus
    def __init__(
        self,
        id: _Optional[str] = ...,
        symbol: _Optional[str] = ...,
        amount: _Optional[int] = ...,
        filled: _Optional[int] = ...,
        commission: _Optional[float] = ...,
        limit_price: _Optional[float] = ...,
        stop_price: _Optional[float] = ...,
        created_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        status: _Optional[_Union[OrderStatus, str]] = ...,
    ) -> None: ...
