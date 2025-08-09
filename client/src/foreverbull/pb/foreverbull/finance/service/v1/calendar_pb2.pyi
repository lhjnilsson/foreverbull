from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from foreverbull.pb.foreverbull.finance.v1 import calendar_pb2 as _calendar_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListCalendarDatesRequest(_message.Message):
    __slots__ = ("code", "start", "end")
    CODE_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    code: str
    start: _common_pb2.Date
    end: _common_pb2.Date
    def __init__(self, code: _Optional[str] = ..., start: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end: _Optional[_Union[_common_pb2.Date, _Mapping]] = ...) -> None: ...

class ListCalendarDatesResponse(_message.Message):
    __slots__ = ("dates",)
    DATES_FIELD_NUMBER: _ClassVar[int]
    dates: _containers.RepeatedCompositeFieldContainer[_calendar_pb2.CalendarDate]
    def __init__(self, dates: _Optional[_Iterable[_Union[_calendar_pb2.CalendarDate, _Mapping]]] = ...) -> None: ...
