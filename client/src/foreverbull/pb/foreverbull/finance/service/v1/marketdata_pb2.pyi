from foreverbull.pb.buf.validate import validate_pb2 as _validate_pb2
from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2 as _marketdata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetAssetRequest(_message.Message):
    __slots__ = ("symbol", "isin")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    ISIN_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    isin: str
    def __init__(self, symbol: _Optional[str] = ..., isin: _Optional[str] = ...) -> None: ...

class GetAssetResponse(_message.Message):
    __slots__ = ("asset",)
    ASSET_FIELD_NUMBER: _ClassVar[int]
    asset: _marketdata_pb2.Asset
    def __init__(self, asset: _Optional[_Union[_marketdata_pb2.Asset, _Mapping]] = ...) -> None: ...

class GetIndexRequest(_message.Message):
    __slots__ = ("symbol",)
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    def __init__(self, symbol: _Optional[str] = ...) -> None: ...

class GetIndexResponse(_message.Message):
    __slots__ = ("assets",)
    ASSETS_FIELD_NUMBER: _ClassVar[int]
    assets: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Asset]
    def __init__(self, assets: _Optional[_Iterable[_Union[_marketdata_pb2.Asset, _Mapping]]] = ...) -> None: ...

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
    __slots__ = ("symbol", "status", "error")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[DownloadHistoricalDataResponse.Status]
        STATUS_OK: _ClassVar[DownloadHistoricalDataResponse.Status]
        STATUS_ERROR: _ClassVar[DownloadHistoricalDataResponse.Status]
    STATUS_UNSPECIFIED: DownloadHistoricalDataResponse.Status
    STATUS_OK: DownloadHistoricalDataResponse.Status
    STATUS_ERROR: DownloadHistoricalDataResponse.Status
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    status: DownloadHistoricalDataResponse.Status
    error: str
    def __init__(self, symbol: _Optional[str] = ..., status: _Optional[_Union[DownloadHistoricalDataResponse.Status, str]] = ..., error: _Optional[str] = ...) -> None: ...
