from datetime import date
from datetime import datetime
from datetime import timezone
from typing import Any
from typing import TypeVar

import pandas

from protobuf.wkt import Struct
from protobuf.wkt import Timestamp
from protobuf.wkt import Value

from foreverbull.pb.foreverbull import common_pb

_T = TypeVar("_T")


def required(value: _T | None, what: str) -> _T:
    if value is None:
        raise ValueError(f"missing {what}")
    return value


def from_proto_date_to_pydate(d: common_pb.Date | None) -> date:
    if d is None:
        raise ValueError("missing date")
    return date(year=d.year, month=d.month, day=d.day)


def from_proto_date_to_pandas_timestamp(d: common_pb.Date, normalize=True, tz_localize: bool | None = False) -> Any:
    timestamp = pandas.Timestamp(year=d.year, month=d.month, day=d.day)
    assert type(timestamp) is pandas.Timestamp, f"Expected pandas.Timestamp, got {type(timestamp)}"
    if normalize:
        timestamp = timestamp.normalize()
    if tz_localize or tz_localize is None:
        timestamp = timestamp.tz_localize(tz_localize)
    return timestamp


def from_pydate_to_proto_date(d: date | datetime) -> common_pb.Date:
    return common_pb.Date(year=d.year, month=d.month, day=d.day)


def from_proto_timestamp(timestamp: Timestamp | None, tz=timezone.utc) -> datetime:
    if timestamp is None:
        raise ValueError("missing timestamp")
    return datetime.fromtimestamp(timestamp.seconds + timestamp.nanos / 1e9, tz=tz)


def to_proto_timestamp(dt: datetime) -> Timestamp:
    return Timestamp(seconds=int(dt.timestamp()), nanos=int(dt.microsecond * 1e3))


def _struct_value_to_native(v: Value):
    # NullValue is 0, so presence is the oneof field name, not the numeric value.
    kind = v.kind
    if kind is None:
        raise ValueError("Unknown value type")
    match kind.field:
        case "null_value":
            return None
        case "number_value" | "string_value" | "bool_value":
            return kind.value
        case "struct_value":
            return protobuf_struct_to_dict(kind.value)
        case _:
            raise ValueError("Unknown value type")


def protobuf_struct_to_dict(struct: Struct) -> dict[str, Any]:
    return {k: _struct_value_to_native(v) for k, v in struct.fields.items()}
