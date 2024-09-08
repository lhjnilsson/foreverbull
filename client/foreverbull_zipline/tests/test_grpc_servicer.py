import time

import grpc
import pytest
from foreverbull.entity import backtest
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, engine_pb2_grpc
from foreverbull_zipline.engine import EngineProcess
from foreverbull_zipline.grpc_servicer import serve


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
    service = serve(engine)
    service.start()
    time.sleep(1)
    yield
    service.stop(None)


@pytest.fixture
def stub():
    return engine_pb2_grpc.EngineStub(grpc.insecure_channel("localhost:50055"))


def test_ingest(
    stub: engine_pb2_grpc.EngineStub,
    servicer,
    engine,
    backtest_entity: backtest.Backtest,
):
    response = stub.Ingest(
        backtest_pb2.IngestRequest(
            ingestion=backtest_pb2.Ingestion(
                start_date=pb_utils.to_proto_timestamp(backtest_entity.start),
                end_date=pb_utils.to_proto_timestamp(backtest_entity.end),
                symbols=backtest_entity.symbols,
            )
        )
    )
    assert response.ingestion.start_date == pb_utils.to_proto_timestamp(backtest_entity.start)
    assert response.ingestion.end_date == pb_utils.to_proto_timestamp(backtest_entity.end)
    assert response.ingestion.symbols == backtest_entity.symbols


def test_run_and_get_result(
    stub: engine_pb2_grpc.EngineStub,
    servicer,
    engine,
    execution: backtest.Execution,
):
    response = stub.RunBacktest(
        backtest_pb2.RunRequest(
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

    period = stub.GetNextPeriod(backtest_pb2.GetNextPeriodRequest())
    assert period.is_running is True

    stub.PlaceOrders(
        backtest_pb2.PlaceOrdersRequest(
            orders=[
                backtest_pb2.Order(
                    symbol="AAPL",
                    amount=1,
                )
            ]
        )
    )

    period = stub.GetNextPeriod(backtest_pb2.GetNextPeriodRequest())
    assert period.is_running is True
    assert period.portfolio.positions[0].symbol == "AAPL"
    assert period.portfolio.positions[0].amount == 1

    while True:
        period = stub.GetNextPeriod(backtest_pb2.GetNextPeriodRequest())
        if period.is_running is False:
            break

    result = stub.GetResult(backtest_pb2.GetResultRequest())
    assert result.periods
