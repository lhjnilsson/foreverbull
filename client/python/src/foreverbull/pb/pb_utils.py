from datetime import datetime, timezone

from google.protobuf.timestamp_pb2 import Timestamp


def from_proto_timestamp(timestamp: Timestamp, tz=timezone.utc) -> datetime:
    return datetime.fromtimestamp(timestamp.seconds + timestamp.nanos / 1e9, tz=tz)


def to_proto_timestamp(dt: datetime) -> Timestamp:
    return Timestamp(seconds=int(dt.timestamp()), nanos=int(dt.microsecond * 1e3))
