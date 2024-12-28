from datetime import date, datetime, timezone
from typing import Any

import pandas
from foreverbull.pb.foreverbull import common_pb2
from google.protobuf.struct_pb2 import Struct
from google.protobuf.timestamp_pb2 import Timestamp


def from_proto_date_to_pydate(d: common_pb2.Date) -> date:
    return date(year=d.year, month=d.month, day=d.day)


def from_proto_date_to_pandas_timestamp(d: common_pb2.Date, normalize=True, tz_localize: bool | None =False) -> Any:
    timestamp = pandas.Timestamp(year=d.year, month=d.month, day=d.day)
    assert type(timestamp) is pandas.Timestamp, f"Expected pandas.Timestamp, got {type(timestamp)}"
    if normalize:
       timestamp = timestamp.normalize()
    if tz_localize or tz_localize is None:
        timestamp = timestamp.tz_localize(tz_localize)
    return timestamp

def from_pydate_to_proto_date(d: date | datetime) -> common_pb2.Date:
    return common_pb2.Date(year=d.year, month=d.month, day=d.day)


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
