import time

import grpc
import pytest
from foreverbull.entity import backtest
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest import (
    backtest_pb2,
    engine_service_pb2,
    engine_service_pb2_grpc,
    ingestion_pb2,
)
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_zipline import grpc_servicer
from foreverbull_zipline.engine import EngineProcess


@pytest.fixture
def engine(fb_database):
    e = EngineProcess()
    e.start()
    e.is_ready.wait(3.0)
    yield e
    e.stop()
    e.join(3.0)


@pytest.fixture
def servicer(engine):
    with grpc_servicer.grpc_server(engine) as server:
        time.sleep(1)  # wait for server to start, abit hacky
        yield server


@pytest.fixture
def stub(servicer):
    return engine_service_pb2_grpc.EngineStub(grpc.insecure_channel("localhost:50055"))


def test_ingest(
    stub: engine_service_pb2_grpc.EngineStub,
    backtest_entity: backtest.Backtest,
):
    response = stub.Ingest(
        engine_service_pb2.IngestRequest(
            ingestion=ingestion_pb2.Ingestion(
                start_date=pb_utils.to_proto_timestamp(backtest_entity.start),
                end_date=pb_utils.to_proto_timestamp(backtest_entity.end),
                symbols=backtest_entity.symbols,
            )
        )
    )
    assert response.ingestion.start_date == pb_utils.to_proto_timestamp(
        backtest_entity.start
    )
    assert response.ingestion.end_date == pb_utils.to_proto_timestamp(
        backtest_entity.end
    )
    assert response.ingestion.symbols == backtest_entity.symbols


def test_run_and_get_result(
    stub: engine_service_pb2_grpc.EngineStub,
    execution: backtest.Execution,
):
    response = stub.RunBacktest(
        engine_service_pb2.RunRequest(
            backtest=backtest_pb2.Backtest(
                start_date=pb_utils.to_proto_timestamp(execution.start),
                end_date=pb_utils.to_proto_timestamp(execution.end),
                symbols=execution.symbols,
            )
        )
    )
    assert response.backtest.start_date == pb_utils.to_proto_timestamp(execution.start)
    assert response.backtest.end_date == pb_utils.to_proto_timestamp(execution.end)
    assert response.backtest.symbols == execution.symbols

    period = stub.GetCurrentPeriod(engine_service_pb2.GetCurrentPeriodRequest())
    assert period.is_running is True

    stub.PlaceOrdersAndContinue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(
                    symbol="AAPL",
                    amount=1,
                )
            ]
        )
    )

    period = stub.GetCurrentPeriod(engine_service_pb2.GetCurrentPeriodRequest())
    assert period.is_running is True
    assert period.portfolio.positions[0].symbol == "AAPL"
    assert period.portfolio.positions[0].amount == 1

    while True:
        period = stub.GetCurrentPeriod(engine_service_pb2.GetCurrentPeriodRequest())
        if period.is_running is False:
            break
        stub.PlaceOrdersAndContinue(engine_service_pb2.PlaceOrdersAndContinueRequest())

    result = stub.GetResult(engine_service_pb2.GetResultRequest())
    assert result.periods
