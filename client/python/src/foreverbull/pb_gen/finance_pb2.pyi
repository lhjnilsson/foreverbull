"""
@generated by mypy-protobuf.  Do not edit manually!
isort:skip_file
"""

import builtins
import collections.abc
import google.protobuf.descriptor
import google.protobuf.internal.containers
import google.protobuf.internal.enum_type_wrapper
import google.protobuf.message
import google.protobuf.timestamp_pb2
import sys
import typing

if sys.version_info >= (3, 10):
    import typing as typing_extensions
else:
    import typing_extensions

DESCRIPTOR: google.protobuf.descriptor.FileDescriptor

class _OrderStatus:
    ValueType = typing.NewType("ValueType", builtins.int)
    V: typing_extensions.TypeAlias = ValueType

class _OrderStatusEnumTypeWrapper(
    google.protobuf.internal.enum_type_wrapper._EnumTypeWrapper[_OrderStatus.ValueType], builtins.type
):
    DESCRIPTOR: google.protobuf.descriptor.EnumDescriptor
    OPEN: _OrderStatus.ValueType  # 0
    FILLED: _OrderStatus.ValueType  # 1
    CANCELED: _OrderStatus.ValueType  # 2
    REJECTED: _OrderStatus.ValueType  # 3
    HELD: _OrderStatus.ValueType  # 4

class OrderStatus(_OrderStatus, metaclass=_OrderStatusEnumTypeWrapper): ...

OPEN: OrderStatus.ValueType  # 0
FILLED: OrderStatus.ValueType  # 1
CANCELED: OrderStatus.ValueType  # 2
REJECTED: OrderStatus.ValueType  # 3
HELD: OrderStatus.ValueType  # 4
global___OrderStatus = OrderStatus

@typing.final
class Position(google.protobuf.message.Message):
    DESCRIPTOR: google.protobuf.descriptor.Descriptor

    SYMBOL_FIELD_NUMBER: builtins.int
    EXCHANGE_FIELD_NUMBER: builtins.int
    AMOUNT_FIELD_NUMBER: builtins.int
    COST_FIELD_NUMBER: builtins.int
    SIDE_FIELD_NUMBER: builtins.int
    symbol: builtins.str
    exchange: builtins.str
    amount: builtins.float
    cost: builtins.float
    side: builtins.str
    def __init__(
        self,
        *,
        symbol: builtins.str = ...,
        exchange: builtins.str = ...,
        amount: builtins.float = ...,
        cost: builtins.float = ...,
        side: builtins.str = ...,
    ) -> None: ...
    def ClearField(
        self,
        field_name: typing.Literal[
            "amount", b"amount", "cost", b"cost", "exchange", b"exchange", "side", b"side", "symbol", b"symbol"
        ],
    ) -> None: ...

global___Position = Position

@typing.final
class Portfolio(google.protobuf.message.Message):
    DESCRIPTOR: google.protobuf.descriptor.Descriptor

    CASH_FIELD_NUMBER: builtins.int
    VALUE_FIELD_NUMBER: builtins.int
    POSITIONS_FIELD_NUMBER: builtins.int
    cash: builtins.float
    value: builtins.float
    @property
    def positions(self) -> google.protobuf.internal.containers.RepeatedCompositeFieldContainer[global___Position]: ...
    def __init__(
        self,
        *,
        cash: builtins.float = ...,
        value: builtins.float = ...,
        positions: collections.abc.Iterable[global___Position] | None = ...,
    ) -> None: ...
    def ClearField(
        self, field_name: typing.Literal["cash", b"cash", "positions", b"positions", "value", b"value"]
    ) -> None: ...

global___Portfolio = Portfolio

@typing.final
class Order(google.protobuf.message.Message):
    DESCRIPTOR: google.protobuf.descriptor.Descriptor

    ID_FIELD_NUMBER: builtins.int
    SYMBOL_FIELD_NUMBER: builtins.int
    AMOUNT_FIELD_NUMBER: builtins.int
    FILLED_FIELD_NUMBER: builtins.int
    COMMISSION_FIELD_NUMBER: builtins.int
    LIMIT_PRICE_FIELD_NUMBER: builtins.int
    STOP_PRICE_FIELD_NUMBER: builtins.int
    CREATED_AT_FIELD_NUMBER: builtins.int
    STATUS_FIELD_NUMBER: builtins.int
    id: builtins.str
    symbol: builtins.str
    amount: builtins.int
    filled: builtins.int
    commission: builtins.float
    limit_price: builtins.float
    stop_price: builtins.float
    status: global___OrderStatus.ValueType
    @property
    def created_at(self) -> google.protobuf.timestamp_pb2.Timestamp: ...
    def __init__(
        self,
        *,
        id: builtins.str = ...,
        symbol: builtins.str = ...,
        amount: builtins.int = ...,
        filled: builtins.int = ...,
        commission: builtins.float = ...,
        limit_price: builtins.float = ...,
        stop_price: builtins.float = ...,
        created_at: google.protobuf.timestamp_pb2.Timestamp | None = ...,
        status: global___OrderStatus.ValueType = ...,
    ) -> None: ...
    def HasField(self, field_name: typing.Literal["created_at", b"created_at"]) -> builtins.bool: ...
    def ClearField(
        self,
        field_name: typing.Literal[
            "amount",
            b"amount",
            "commission",
            b"commission",
            "created_at",
            b"created_at",
            "filled",
            b"filled",
            "id",
            b"id",
            "limit_price",
            b"limit_price",
            "status",
            b"status",
            "stop_price",
            b"stop_price",
            "symbol",
            b"symbol",
        ],
    ) -> None: ...

global___Order = Order