from datetime import datetime
from datetime import timezone

from google.protobuf.timestamp_pb2 import Timestamp

from foreverbull.pb import pb_utils


def test_from_proto_timestamp():
    ts = Timestamp()
    ts.seconds = 1704110400
    assert pb_utils.from_proto_timestamp(ts) == datetime(2024, 1, 1, 12, 0, tzinfo=timezone.utc)


def test_to_proto_timestamp():
    dt = datetime(2024, 1, 1, 12, 0, tzinfo=timezone.utc)
    ts = Timestamp()
    ts.seconds = 1704110400
    assert pb_utils.to_proto_timestamp(dt) == ts
