from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class IngestionStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CREATED: _ClassVar[IngestionStatus]
    INGESTING: _ClassVar[IngestionStatus]
    READY: _ClassVar[IngestionStatus]
CREATED: IngestionStatus
INGESTING: IngestionStatus
READY: IngestionStatus

class Ingestion(_message.Message):
    __slots__ = ("start_date", "end_date", "symbols")
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    start_date: _common_pb2.Date
    end_date: _common_pb2.Date
    symbols: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., symbols: _Optional[_Iterable[str]] = ...) -> None: ...
