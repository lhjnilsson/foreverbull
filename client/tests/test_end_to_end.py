import os
import time
from datetime import datetime, timezone
from functools import partial
from multiprocessing import get_start_method, set_start_method
from typing import Generator

import pandas as pd
import pynng
import pytest
from foreverbull import Algorithm, entity
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, engine_pb2
from foreverbull.pb.finance import finance_pb2
from foreverbull.pb.service import service_pb2, worker_pb2
from foreverbull_zipline import engine, grpc_servicer
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from zipline.api import order_target, symbol
from zipline.data import bundles
from zipline.data.bundles import register
from zipline.errors import SymbolNotFound
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.run_algo import BenchmarkSpec, _run


@pytest.mark.parametrize("file_path", ["tests/end_to_end/parallel.py"])
def test_integration(engine_stub, execution, foreverbull_bundle, baseline_performance, file_path):
    broker_socket = pynng.Req0(listen="tcp://0.0.0.0:8888", recv_timeout=10_000, send_timeout=10_000)
    namespace_socket = pynng.Rep0(listen="tcp://0.0.0.0:9999", recv_timeout=10_000, send_timeout=10_000)

    configure_request = worker_pb2.ConfigureExecutionRequest(
        configuration=service_pb2.ExecutionConfiguration(
            brokerPort=8888,
            namespacePort=9999,
            databaseURL=os.environ["DATABASE_URL"],
            functions=[],
        )
    )

    algorithm = Algorithm.from_file_path(file_path)
    with algorithm.backtest_session("asdf") as backtest:
        algorithm.run_execution(backtest)

        while True:
            request = service_pb2.Request(task="get_portfolio")
            backtest.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(backtest.recv())
            if response.HasField("error"):
                assert response.error == "no active execution"
                break
            portfolio = engine_pb2.GetPortfolioResponse()
            portfolio.ParseFromString(response.data)

            p = finance_pb2.Portfolio(
                cash=portfolio.starting_cash,
                value=portfolio.portfolio_value,
                positions=[
                    finance_pb2.Position(symbol=p.symbol, exchange="backtest", amount=p.amount, cost=p.cost_basis)
                    for p in portfolio.positions
                ],
            )

            request = service_pb2.WorkerRequest(
                task="handle_data",
                timestamp=portfolio.timestamp,
                symbols=execution.symbols,
                portfolio=p,
            )
            broker_socket.send(request.SerializeToString())
            response = service_pb2.WorkerResponse()
            response.ParseFromString(broker_socket.recv())
            if response.HasField("error"):
                raise Exception(response.error)

            continue_request = engine_pb2.ContinueRequest(orders=[])
            for o in response.orders:
                continue_request.orders.append(
                    engine_pb2.Order(
                        symbol=o.symbol,
                        amount=o.amount,
                    )
                )

            request = service_pb2.Request(task="continue", data=continue_request.SerializeToString())
            backtest.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(backtest.recv())
            if response.HasField("error"):
                raise Exception(response.error)

        request = service_pb2.Request(task="get_execution_result")
        backtest.send(request.SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(backtest.recv())
        assert response.HasField("error") is False
        result_response = engine_pb2.ResultResponse()
        result_response.ParseFromString(response.data)
        p = []
        for period in result_response.periods:
            p.append(
                {
                    "portfolio_value": period.portfolio_value,
                    "returns": period.returns,
                    "alpha": period.alpha if period.HasField("alpha") else None,
                    "beta": period.beta if period.HasField("beta") else None,
                    "sharpe": period.sharpe if period.HasField("sharpe") else None,
                }
            )
        result = pd.DataFrame(p).reset_index(drop=True)
        baseline_performance = baseline_performance[result.columns]
        assert baseline_performance.equals(result)

    broker_socket.close()
    namespace_socket.close()
