from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as _finance_pb2
from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetAssetRequest(_message.Message):
    __slots__ = ("symbol",)
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    def __init__(self, symbol: _Optional[str] = ...) -> None: ...

class GetAssetResponse(_message.Message):
    __slots__ = ("asset",)
    ASSET_FIELD_NUMBER: _ClassVar[int]
    asset: _finance_pb2.Asset
    def __init__(self, asset: _Optional[_Union[_finance_pb2.Asset, _Mapping]] = ...) -> None: ...

class GetIndexRequest(_message.Message):
    __slots__ = ("symbol",)
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    def __init__(self, symbol: _Optional[str] = ...) -> None: ...

class GetIndexResponse(_message.Message):
    __slots__ = ("assets",)
    ASSETS_FIELD_NUMBER: _ClassVar[int]
    assets: _containers.RepeatedCompositeFieldContainer[_finance_pb2.Asset]
    def __init__(self, assets: _Optional[_Iterable[_Union[_finance_pb2.Asset, _Mapping]]] = ...) -> None: ...

class DownloadHistoricalDataRequest(_message.Message):
    __slots__ = ("symbols", "start_date", "end_date")
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    symbols: _containers.RepeatedScalarFieldContainer[str]
    start_date: _common_pb2.Date
    end_date: _common_pb2.Date
    def __init__(self, symbols: _Optional[_Iterable[str]] = ..., start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ...) -> None: ...

class DownloadHistoricalDataResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
