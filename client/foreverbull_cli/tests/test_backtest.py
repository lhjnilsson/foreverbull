import tempfile
import traceback

from datetime import date
from datetime import datetime
from unittest.mock import patch

from typer.testing import CliRunner

from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull.pb.pb_utils import to_proto_timestamp
from foreverbull_cli.backtest import backtest


runner = CliRunner(mix_stderr=False)


def test_backtest_list():
    with patch("foreverbull.broker.backtest.list_backtests") as mock_list:
        mock_list.return_value = [
            backtest_pb2.Backtest(
                name="test_name",
                start_date=from_pydate_to_proto_date(date.today()),
                end_date=from_pydate_to_proto_date(date.today()),
                symbols=["AAPL", "MSFT"],
                statuses=[
                    backtest_pb2.Backtest.Status(
                        status=backtest_pb2.Backtest.Status.Status.READY,
                        error=None,
                        occurred_at=to_proto_timestamp(datetime.now()),
                    )
                ],
            )
        ]
        result = runner.invoke(backtest, ["list"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_create():
    with (
        patch("foreverbull.broker.backtest.create") as mock_create,
        tempfile.NamedTemporaryFile() as cfg_file,
    ):
        mock_create.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=from_pydate_to_proto_date(date.today()),
            end_date=from_pydate_to_proto_date(date.today()),
            symbols=["AAPL", "MSFT"],
            statuses=[
                backtest_pb2.Backtest.Status(
                    status=backtest_pb2.Backtest.Status.Status.CREATED,
                    error=None,
                    occurred_at=to_proto_timestamp(datetime.now()),
                )
            ],
        )
        cfg_file.write(b'{"start_date": "2021-01-01", "end_date": "2021-01-31", "symbols": ["AAPL", "MSFT"]}')
        cfg_file.flush()
        result = runner.invoke(
            backtest,
            [
                "create",
                cfg_file.name,
            ],
        )

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)  # type: ignore
        assert "test_name" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_get():
    with (
        patch("foreverbull.broker.backtest.get") as mock_get,
    ):
        mock_get.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=from_pydate_to_proto_date(date.today()),
            end_date=from_pydate_to_proto_date(date.today()),
            symbols=["AAPL", "MSFT"],
            statuses=[
                backtest_pb2.Backtest.Status(
                    status=backtest_pb2.Backtest.Status.Status.READY,
                    error=None,
                    occurred_at=to_proto_timestamp(datetime.now()),
                )
            ],
        )
        result = runner.invoke(backtest, ["get", "test"])

        if not result.exit_code == 0 and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert "test" in result.stdout
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout
