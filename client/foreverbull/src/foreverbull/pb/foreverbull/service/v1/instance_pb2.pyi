from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Instance(_message.Message):
    __slots__ = ("ID", "Image", "Host", "Port", "statuses")
    class Status(_message.Message):
        __slots__ = ("status", "error", "OccurredAt")
        class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
            __slots__ = ()
            CREATED: _ClassVar[Instance.Status.Status]
            RUNNING: _ClassVar[Instance.Status.Status]
            CONFIGURED: _ClassVar[Instance.Status.Status]
            EXECUTING: _ClassVar[Instance.Status.Status]
            COMPLETED: _ClassVar[Instance.Status.Status]
            ERROR: _ClassVar[Instance.Status.Status]
        CREATED: Instance.Status.Status
        RUNNING: Instance.Status.Status
        CONFIGURED: Instance.Status.Status
        EXECUTING: Instance.Status.Status
        COMPLETED: Instance.Status.Status
        ERROR: Instance.Status.Status
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        OCCURREDAT_FIELD_NUMBER: _ClassVar[int]
        status: Instance.Status.Status
        error: str
        OccurredAt: _timestamp_pb2.Timestamp
        def __init__(self, status: _Optional[_Union[Instance.Status.Status, str]] = ..., error: _Optional[str] = ..., OccurredAt: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    ID: str
    Image: str
    Host: str
    Port: int
    statuses: _containers.RepeatedCompositeFieldContainer[Instance.Status]
    def __init__(self, ID: _Optional[str] = ..., Image: _Optional[str] = ..., Host: _Optional[str] = ..., Port: _Optional[int] = ..., statuses: _Optional[_Iterable[_Union[Instance.Status, _Mapping]]] = ...) -> None: ...
