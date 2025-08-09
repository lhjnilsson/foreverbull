from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class Session(_message.Message):
    __slots__ = ("id", "port")
    ID_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    id: str
    port: int
    def __init__(self, id: _Optional[str] = ..., port: _Optional[int] = ...) -> None: ...
