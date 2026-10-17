from pathlib import Path

from foreverbull.pb.foreverbull.backtest.v1 import execution_pb


# Bytes are proto.Marshal output from Go. Do not replace this file from Python.
_FIXTURE = Path(__file__).parent / "testdata" / "backtest_period.bin"


def test_go_period_payload():
    period = execution_pb.Period.from_binary(_FIXTURE.read_bytes())

    date = period.date
    assert date is not None
    assert (date.year, date.month, date.day) == (2024, 1, 2)
    assert period.PNL == 12.5
    assert period.portfolio_value == 10050.25
    assert period.has_field("sharpe") is True
    assert period.sharpe == 1.25
    assert period.has_field("alpha") is False

    assert len(period.positions) == 2
    aapl = period.positions[0]
    assert aapl.symbol == "AAPL"
    assert aapl.amount == 10
    assert aapl.cost_basis == 150.5
    assert aapl.last_sale_price == 155
    assert aapl.isin == "US0378331005"
    sale = aapl.last_sale_date
    assert sale is not None
    assert sale.seconds == 1704229200
    assert sale.nanos == 0

    ibm = period.positions[1]
    assert ibm.symbol == "IBM"
    assert ibm.amount == -4
    assert ibm.has_field("last_sale_date") is False
