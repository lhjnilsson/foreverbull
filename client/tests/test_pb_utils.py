from datetime import UTC
from datetime import date
from datetime import datetime

import pandas
import pytest

from protobuf.wkt import Struct
from protobuf.wkt import Timestamp
from protobuf.wkt import Value

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb


def test_from_proto_timestamp():
    ts = Timestamp()
    ts.seconds = 1704110400
    assert pb_utils.from_proto_timestamp(ts) == datetime(2024, 1, 1, 12, 0, tzinfo=UTC)


def test_to_proto_timestamp():
    dt = datetime(2024, 1, 1, 12, 0, tzinfo=UTC)
    ts = Timestamp()
    ts.seconds = 1704110400
    assert pb_utils.to_proto_timestamp(dt) == ts


def test_proto_date_roundtrip():
    src = date(2024, 3, 9)
    proto = pb_utils.from_pydate_to_proto_date(src)
    assert proto == common_pb.Date(year=2024, month=3, day=9)
    assert pb_utils.from_proto_date_to_pydate(proto) == src
    assert pb_utils.from_pydate_to_proto_date(datetime(2024, 3, 9, 15, 4, 5)) == proto


def test_from_proto_date_to_pandas_timestamp():
    proto = common_pb.Date(year=2024, month=3, day=9)
    ts = pb_utils.from_proto_date_to_pandas_timestamp(proto)
    assert isinstance(ts, pandas.Timestamp)
    assert ts == pandas.Timestamp("2024-03-09")
    assert ts.tzinfo is None

    localized = pb_utils.from_proto_date_to_pandas_timestamp(proto, tz_localize=None)
    assert localized.tzinfo is None


def test_protobuf_struct_to_dict():
    struct = Struct.from_python(
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

    lists = Struct.from_python({"nums": [1, 2]})
    with pytest.raises(ValueError, match="Unknown value type"):
        pb_utils.protobuf_struct_to_dict(lists)

    empty = Struct()
    empty.fields["unset"] = Value()
    with pytest.raises(ValueError, match="Unknown value type"):
        pb_utils.protobuf_struct_to_dict(empty)


def test_backtest_period_enabled_assets_and_dates():
    entity = backtest_pb.Backtest(
        name="demo",
        benchmark="SPY",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=pb_utils.from_pydate_to_proto_date(date(2020, 1, 2)),
                end_date=pb_utils.from_pydate_to_proto_date(date(2020, 6, 30)),
                assets=[
                    backtest_pb.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="IBM", enabled=False),
                ],
            ),
            backtest_pb.Backtest.Period(
                start_date=pb_utils.from_pydate_to_proto_date(date(2020, 7, 1)),
                end_date=pb_utils.from_pydate_to_proto_date(date(2020, 12, 31)),
                assets=[
                    backtest_pb.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="MSFT", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="IBM", enabled=False),
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

    parsed = backtest_pb.Backtest.from_binary(entity.to_binary())
    assert parsed.name == "demo"
    assert parsed.benchmark == "SPY"
    assert parsed.periods[0].assets[1].enabled is False
