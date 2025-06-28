from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CalendarDate(_message.Message):
    __slots__ = ("date", "open", "close")
    DATE_FIELD_NUMBER: _ClassVar[int]
    OPEN_FIELD_NUMBER: _ClassVar[int]
    CLOSE_FIELD_NUMBER: _ClassVar[int]
    date: _common_pb2.Date
    open: _timestamp_pb2.Timestamp
    close: _timestamp_pb2.Timestamp
    def __init__(self, date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., open: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., close: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
