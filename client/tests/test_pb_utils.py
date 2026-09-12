from datetime import date
from datetime import datetime
from datetime import timezone

import pandas
import pytest

from google.protobuf.struct_pb2 import Struct
from google.protobuf.struct_pb2 import Value
from google.protobuf.timestamp_pb2 import Timestamp

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2


def test_from_proto_timestamp():
    ts = Timestamp()
    ts.seconds = 1704110400
    assert pb_utils.from_proto_timestamp(ts) == datetime(2024, 1, 1, 12, 0, tzinfo=timezone.utc)


def test_to_proto_timestamp():
    dt = datetime(2024, 1, 1, 12, 0, tzinfo=timezone.utc)
    ts = Timestamp()
    ts.seconds = 1704110400
    assert pb_utils.to_proto_timestamp(dt) == ts


def test_proto_date_roundtrip():
    src = date(2024, 3, 9)
    proto = pb_utils.from_pydate_to_proto_date(src)
    assert proto == common_pb2.Date(year=2024, month=3, day=9)
    assert pb_utils.from_proto_date_to_pydate(proto) == src
    assert pb_utils.from_pydate_to_proto_date(datetime(2024, 3, 9, 15, 4, 5)) == proto


def test_from_proto_date_to_pandas_timestamp():
    proto = common_pb2.Date(year=2024, month=3, day=9)
    ts = pb_utils.from_proto_date_to_pandas_timestamp(proto)
    assert isinstance(ts, pandas.Timestamp)
    assert ts == pandas.Timestamp("2024-03-09")
    assert ts.tzinfo is None

    localized = pb_utils.from_proto_date_to_pandas_timestamp(proto, tz_localize=None)
    assert localized.tzinfo is None


def test_protobuf_struct_to_dict():
    struct = Struct()
    struct.update(
        {
            "n": 1.5,
            "s": "hello",
            "b": True,
            "z": None,
            "nested": {"k": "v"},
        }
    )
    assert pb_utils.protobuf_struct_to_dict(struct) == {
        "n": 1.5,
        "s": "hello",
        "b": True,
        "z": None,
        "nested": {"k": "v"},
    }

    lists = Struct()
    lists.update({"nums": [1, 2]})
    with pytest.raises(ValueError, match="Unknown value type"):
        pb_utils.protobuf_struct_to_dict(lists)

    empty = Struct()
    empty.fields["unset"].CopyFrom(Value())
    with pytest.raises(ValueError, match="Unknown value type"):
        pb_utils.protobuf_struct_to_dict(empty)


def test_backtest_period_enabled_assets_and_dates():
    entity = backtest_pb2.Backtest(
        name="demo",
        benchmark="SPY",
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=pb_utils.from_pydate_to_proto_date(date(2020, 1, 2)),
                end_date=pb_utils.from_pydate_to_proto_date(date(2020, 6, 30)),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="IBM", enabled=False),
                ],
            ),
            backtest_pb2.Backtest.Period(
                start_date=pb_utils.from_pydate_to_proto_date(date(2020, 7, 1)),
                end_date=pb_utils.from_pydate_to_proto_date(date(2020, 12, 31)),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="MSFT", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="IBM", enabled=False),
                ],
            ),
        ],
    )
    enabled: list[str] = []
    seen: set[str] = set()
    for period in entity.periods:
        for asset in period.assets:
            if asset.enabled and asset.symbol not in seen:
                seen.add(asset.symbol)
                enabled.append(asset.symbol)
    assert enabled == ["AAPL", "MSFT"]
    assert pb_utils.from_proto_date_to_pydate(entity.periods[0].start_date).isoformat() == "2020-01-02"
    assert pb_utils.from_proto_date_to_pydate(entity.periods[-1].end_date).isoformat() == "2020-12-31"

    parsed = backtest_pb2.Backtest()
    parsed.ParseFromString(entity.SerializeToString())
    assert parsed.name == "demo"
    assert parsed.benchmark == "SPY"
    assert parsed.periods[0].assets[1].enabled is False
