from google.protobuf import timestamp_pb2 as _timestamp_pb2
from foreverbull.pb.foreverbull.service import worker_pb2 as _worker_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Service(_message.Message):
    __slots__ = ("Image", "algorithm", "statuses")
    class Status(_message.Message):
        __slots__ = ("status", "error", "OccurredAt")
        class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
            __slots__ = ()
            CREATED: _ClassVar[Service.Status.Status]
            INTERVIEW: _ClassVar[Service.Status.Status]
            READY: _ClassVar[Service.Status.Status]
            ERROR: _ClassVar[Service.Status.Status]
        CREATED: Service.Status.Status
        INTERVIEW: Service.Status.Status
        READY: Service.Status.Status
        ERROR: Service.Status.Status
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        OCCURREDAT_FIELD_NUMBER: _ClassVar[int]
        status: Service.Status.Status
        error: str
        OccurredAt: _timestamp_pb2.Timestamp
        def __init__(self, status: _Optional[_Union[Service.Status.Status, str]] = ..., error: _Optional[str] = ..., OccurredAt: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    Image: str
    algorithm: _worker_pb2.Algorithm
    statuses: _containers.RepeatedCompositeFieldContainer[Service.Status]
    def __init__(self, Image: _Optional[str] = ..., algorithm: _Optional[_Union[_worker_pb2.Algorithm, _Mapping]] = ..., statuses: _Optional[_Iterable[_Union[Service.Status, _Mapping]]] = ...) -> None: ...
