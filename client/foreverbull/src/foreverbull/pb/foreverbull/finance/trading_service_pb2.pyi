from foreverbull.pb.foreverbull.finance import finance_pb2 as _finance_pb2
from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
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
    portfolio: _finance_pb2.Portfolio
    def __init__(self, portfolio: _Optional[_Union[_finance_pb2.Portfolio, _Mapping]] = ...) -> None: ...

class GetOrdersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetOrdersResponse(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_finance_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_finance_pb2.Order, _Mapping]]] = ...) -> None: ...

class PlaceOrderRequest(_message.Message):
    __slots__ = ("order",)
    ORDER_FIELD_NUMBER: _ClassVar[int]
    order: _finance_pb2.Order
    def __init__(self, order: _Optional[_Union[_finance_pb2.Order, _Mapping]] = ...) -> None: ...

class PlaceOrderResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
