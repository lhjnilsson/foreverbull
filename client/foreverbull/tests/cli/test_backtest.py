import traceback
from datetime import datetime
from unittest.mock import patch

from foreverbull.cli.backtest import backtest
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.pb_utils import to_proto_timestamp
from typer.testing import CliRunner

runner = CliRunner(mix_stderr=False)


def test_backtest_list():
    with patch("foreverbull.broker.backtest.list") as mock_list:
        mock_list.return_value = [
            backtest_pb2.Backtest(
                name="test_name",
                start_date=to_proto_timestamp(datetime.now()),
                end_date=to_proto_timestamp(datetime.now()),
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
        assert "READY" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_create():
    with patch("foreverbull.broker.backtest.create") as mock_create:
        mock_create.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=to_proto_timestamp(datetime.now()),
            end_date=to_proto_timestamp(datetime.now()),
            symbols=["AAPL", "MSFT"],
            statuses=[
                backtest_pb2.Backtest.Status(
                    status=backtest_pb2.Backtest.Status.Status.CREATED,
                    error=None,
                    occurred_at=to_proto_timestamp(datetime.now()),
                )
            ],
        )
        result = runner.invoke(
            backtest,
            [
                "create",
                "test_name",
                "--start",
                "2021-01-01",
                "--end",
                "2021-01-02",
                "--symbols",
                "AAPL",
            ],
        )

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "test_name" in result.stdout
        assert "AAPL,MSFT" in result.stdout


def test_backtest_get():
    with (patch("foreverbull.broker.backtest.get") as mock_get,):
        mock_get.return_value = backtest_pb2.Backtest(
            name="test_name",
            start_date=to_proto_timestamp(datetime.now()),
            end_date=to_proto_timestamp(datetime.now()),
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
