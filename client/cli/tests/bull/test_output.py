from datetime import date

from foreverbull_cli.output import GRAFANA_DASHBOARD_URL
from foreverbull_cli.output import execution_link
from foreverbull_cli.output import format_date_field
from foreverbull_cli.output import format_optional_float
from foreverbull_cli.output import format_proto_date

from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.pb_utils import from_pydate_to_proto_date


def test_format_proto_date():
    proto = from_pydate_to_proto_date(date(2020, 1, 31))
    assert format_proto_date(proto) == "2020-01-31"
    assert format_proto_date(None) == ""
    assert format_proto_date(common_pb2.Date()) == ""
    assert format_proto_date(common_pb2.Date(year=2020, month=0, day=1)) == ""


def test_format_date_field_and_optional_float():
    execution = execution_pb2.Execution(
        id="exec-1",
        start_date=from_pydate_to_proto_date(date(2020, 1, 1)),
    )
    assert format_date_field(execution, "start_date") == "2020-01-01"
    assert format_date_field(execution, "end_date") == ""

    period = execution_pb2.Period(alpha=0.123456)
    assert format_optional_float(period, "alpha") == "0.12346"
    assert format_optional_float(period, "beta") == "-"
    assert execution_link("exec-1") == f"{GRAFANA_DASHBOARD_URL}?var-execution=exec-1"
