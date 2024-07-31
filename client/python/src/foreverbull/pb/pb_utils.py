from datetime import datetime, timezone
from typing import Any

from google.protobuf.struct_pb2 import Struct
from google.protobuf.timestamp_pb2 import Timestamp


def from_proto_timestamp(timestamp: Timestamp, tz=timezone.utc) -> datetime:
    return datetime.fromtimestamp(timestamp.seconds + timestamp.nanos / 1e9, tz=tz)


def to_proto_timestamp(dt: datetime) -> Timestamp:
    return Timestamp(seconds=int(dt.timestamp()), nanos=int(dt.microsecond * 1e3))


def _struct_value_to_native(v):
    if v.HasField("null_value"):
        return None
    elif v.HasField("number_value"):
        return v.number_value
    elif v.HasField("string_value"):
        return v.string_value
    elif v.HasField("bool_value"):
        return v.bool_value
    elif v.HasField("struct_value"):
        return protobuf_struct_to_dict(v.struct_value)
    else:
        raise ValueError("Unknown value type")


def protobuf_struct_to_dict(struct: Struct) -> dict[str, Any]:
    return {k: _struct_value_to_native(v) for k, v in struct.fields.items()}
