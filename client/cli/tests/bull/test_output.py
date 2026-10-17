from datetime import date

from foreverbull_cli.output import GRAFANA_DASHBOARD_UID
from foreverbull_cli.output import GRAFANA_DASHBOARD_URL
from foreverbull_cli.output import execution_link
from foreverbull_cli.output import format_date_field
from foreverbull_cli.output import format_optional_float
from foreverbull_cli.output import format_proto_date
from foreverbull_cli.output import grafana_execution_url

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.pb_utils import from_pydate_to_proto_date


def test_format_proto_date():
    proto = from_pydate_to_proto_date(date(2020, 1, 31))
    assert format_proto_date(proto) == "2020-01-31"
    assert format_proto_date(None) == ""
    assert format_proto_date(common_pb.Date()) == ""
    assert format_proto_date(common_pb.Date(year=2020, month=0, day=1)) == ""


def test_format_date_field_and_optional_float():
    execution = execution_pb.Execution(
        id="exec-1",
        start_date=from_pydate_to_proto_date(date(2020, 1, 1)),
    )
    assert format_date_field(execution, "start_date") == "2020-01-01"
    assert format_date_field(execution, "end_date") == ""

    period = execution_pb.Period(alpha=0.123456)
    assert format_optional_float(period, "alpha") == "0.12346"
    assert format_optional_float(period, "beta") == "-"


def test_execution_link_default(monkeypatch):
    monkeypatch.delenv("GRAFANA_URL", raising=False)
    assert execution_link("exec-1") == f"{GRAFANA_DASHBOARD_URL}?var-execution=exec-1"
    assert execution_link("exec-1") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-1"


def test_grafana_execution_url_builder():
    assert GRAFANA_DASHBOARD_UID == "ee9ep4jxtwkcgf"
    assert grafana_execution_url("", "exec-1") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-1"
    assert (
        grafana_execution_url("http://localhost:3000/", "abc-123")
        == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=abc-123"
    )
    assert (
        grafana_execution_url("https://grafana.example.com", "run-9")
        == "https://grafana.example.com/d/ee9ep4jxtwkcgf?var-execution=run-9"
    )
    assert (
        grafana_execution_url("https://example.com/grafana", "e1")
        == "https://example.com/grafana/d/ee9ep4jxtwkcgf?var-execution=e1"
    )
    assert grafana_execution_url("not a url", "exec-2") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-2"


def test_execution_link_uses_grafana_url_env(monkeypatch):
    monkeypatch.setenv("GRAFANA_URL", "https://grafana.example.com/")
    assert execution_link("exec-1") == "https://grafana.example.com/d/ee9ep4jxtwkcgf?var-execution=exec-1"
    monkeypatch.setenv("GRAFANA_URL", "")
    assert execution_link("exec-1") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-1"
