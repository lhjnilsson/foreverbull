from foreverbull.pb.foreverbull.backtest import ingestion_pb2 as _ingestion_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetCurrentIngestionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCurrentIngestionResponse(_message.Message):
    __slots__ = ("ingestion", "status", "size")
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    ingestion: _ingestion_pb2.Ingestion
    status: _ingestion_pb2.IngestionStatus
    size: int
    def __init__(self, ingestion: _Optional[_Union[_ingestion_pb2.Ingestion, _Mapping]] = ..., status: _Optional[_Union[_ingestion_pb2.IngestionStatus, str]] = ..., size: _Optional[int] = ...) -> None: ...

class UpdateIngestionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class UpdateIngestionResponse(_message.Message):
    __slots__ = ("ingestion", "status")
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ingestion: _ingestion_pb2.Ingestion
    status: _ingestion_pb2.IngestionStatus
    def __init__(self, ingestion: _Optional[_Union[_ingestion_pb2.Ingestion, _Mapping]] = ..., status: _Optional[_Union[_ingestion_pb2.IngestionStatus, str]] = ...) -> None: ...
