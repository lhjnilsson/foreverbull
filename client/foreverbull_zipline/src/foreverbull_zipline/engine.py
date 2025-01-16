import functools
import logging
import logging.handlers
import multiprocessing
import multiprocessing.queues
import os

import pandas as pd
import pynng
import six

from zipline import TradingAlgorithm
from zipline.data import bundles
from zipline.data.bundles.core import BundleData
from zipline.data.data_portal import DataPortal
from zipline.extensions import load
from zipline.finance import metrics
from zipline.finance.blotter import Blotter
from zipline.finance.trading import SimulationParameters
from zipline.protocol import BarData
from zipline.protocol import Portfolio
from zipline.utils.calendar_utils import get_calendar

from foreverbull.broker.storage import Storage
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2


class ConfigError(Exception):
    pass


class StopExcecution(Exception):
    pass


def find_start_timestamp(bundle: BundleData, request: engine_service_pb2.RunBacktestRequest) -> pd.Timestamp:
    bundle_start = None
    for symbol in request.backtest.symbols:
        asset_start = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None).start_date
        bundle_start = asset_start if bundle_start is None else max(bundle_start, asset_start)
    if type(bundle_start) is not pd.Timestamp:
        raise ConfigError("no bundle start_date found")

    if not request.backtest.HasField("start_date"):
        return bundle_start
    request_start = pd.Timestamp(pb_utils.from_proto_date_to_pydate(request.backtest.start_date))
    assert type(request_start) is pd.Timestamp
    if request_start < bundle_start:
        return bundle_start
    return request_start


def find_end_timestamp(bundle: BundleData, request: engine_service_pb2.RunBacktestRequest) -> pd.Timestamp:
    bundle_end = None
    for symbol in request.backtest.symbols:
        asset_end = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None).end_date
        bundle_end = asset_end if bundle_end is None else min(bundle_end, asset_end)
    if type(bundle_end) is not pd.Timestamp:
        raise ConfigError("no bundle end_date found")

    if not request.backtest.HasField("end_date"):
        return bundle_end
    request_end = pd.Timestamp(pb_utils.from_proto_date_to_pydate(request.backtest.end_date))
    assert type(request_end) is pd.Timestamp
    if request_end > bundle_end:
        return bundle_end
    return request_end


class Engine(multiprocessing.Process):
    def __init__(
        self,
        socket_file_path: str = "/tmp/foreverbull_zipline.sock",
        logging_queue: multiprocessing.queues.Queue | None = None,
    ):
        self._socket_file_path = socket_file_path
        self._logging_queue = logging_queue
        self.logger = logging.getLogger().getChild(__name__)
        self.is_ready = multiprocessing.Event()
        super(Engine, self).__init__()

    def run_backtest(self, backtest: engine_service_pb2.RunBacktestRequest) -> engine_service_pb2.RunBacktestResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            data = backtest.SerializeToString()
            request = common_pb2.Request(task="run", data=data)
            socket.send(request.SerializeToString())
            response = common_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            b = engine_service_pb2.RunBacktestResponse()
            b.ParseFromString(response.data)
            return b

    def get_current_period(
        self, req: engine_service_pb2.GetCurrentPeriodRequest
    ) -> engine_service_pb2.GetCurrentPeriodResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            data = req.SerializeToString()
            request = common_pb2.Request(task="get_current_period", data=data)
            socket.send(request.SerializeToString())
            response = common_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            p = engine_service_pb2.GetCurrentPeriodResponse()
            p.ParseFromString(response.data)
            return p

    def place_orders_and_continue(
        self, req: engine_service_pb2.PlaceOrdersAndContinueRequest
    ) -> engine_service_pb2.PlaceOrdersAndContinueResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            data = req.SerializeToString()
            request = common_pb2.Request(task="place_orders_and_continue", data=data)
            socket.send(request.SerializeToString())
            response = common_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            p = engine_service_pb2.PlaceOrdersAndContinueResponse()
            p.ParseFromString(response.data)
            return p

    def get_backtest_result(self, req: engine_service_pb2.GetResultRequest) -> engine_service_pb2.GetResultResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            request = common_pb2.Request(task="get_result")
            socket.send(request.SerializeToString())
            response = common_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            result = engine_service_pb2.GetResultResponse()
            result.ParseFromString(response.data)
            return result

    def stop(self):
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            request = common_pb2.Request(task="stop")
            socket.send(request.SerializeToString())

    def run(self):
        if self._logging_queue is not None:
            handler = logging.handlers.QueueHandler(self._logging_queue)
            logging.basicConfig(handlers=[handler], level=logging.DEBUG)
        self.log = logging.getLogger().getChild(__name__)
        self.log.info("Starting Execution Process")
        self._trading_algorithm: TradingAlgorithm | None = None
        self.logger = logging.getLogger().getChild(__name__)

        if os.path.exists(self._socket_file_path):
            os.remove(self._socket_file_path)
        with pynng.Rep0(
            listen=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            send_timeout=1_000,
        ) as socket:
            self.is_ready.set()
            while True:
                try:
                    self._process_request(None, None, socket)
                except (StopExcecution, KeyboardInterrupt):
                    break

    def analyze(self, _, result):
        self.result = result

    def _run(self, data: bytes, socket: pynng.Socket) -> tuple[TradingAlgorithm, bytes]:
        req = engine_service_pb2.RunBacktestRequest()
        req.ParseFromString(data)
        bundles.register("foreverbull", None, calendar_name="XNYS")
        bundle = bundles.load("foreverbull", os.environ, None)

        for symbol in req.backtest.symbols:
            asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
            if asset is None:
                raise ConfigError(f"Unknown symbol: {symbol}")

        start_date = find_start_timestamp(bundle, req)
        end_date = find_end_timestamp(bundle, req)

        if req.backtest.benchmark:
            benchmark_returns = None
            benchmark_sid = bundle.asset_finder.lookup_symbol(req.backtest.benchmark, as_of_date=None)
        else:
            benchmark_returns = pd.Series(index=pd.date_range(start_date, end_date, tz="utc"), data=0.0)
            benchmark_sid = None

        trading_calendar = get_calendar("XNYS")
        data_portal = DataPortal(
            bundle.asset_finder,
            trading_calendar=trading_calendar,
            first_trading_day=bundle.equity_minute_bar_reader.first_trading_day,
            equity_minute_reader=bundle.equity_minute_bar_reader,
            equity_daily_reader=bundle.equity_daily_bar_reader,
            adjustment_reader=bundle.adjustment_reader,
        )
        sim_params = SimulationParameters(
            start_session=start_date,
            end_session=end_date,
            trading_calendar=trading_calendar,
            data_frequency="daily",
        )
        metrics_set = "default"
        blotter = "default"
        if isinstance(metrics_set, six.string_types):
            try:
                metrics_set = metrics.load(metrics_set)
            except ValueError as e:
                self.logger.error("Error configuring metrics: %s", repr(e))
                raise ConfigError(repr(e))

        if isinstance(blotter, six.string_types):
            try:
                blotter = load(Blotter, blotter)
            except ValueError as e:
                self.logger.error("Error configuring blotter: %s", repr(e))
                raise ConfigError(repr(e))

        handle_data = functools.partial(self._process_request, socket=socket)
        trading_algorithm = TradingAlgorithm(
            namespace={},
            data_portal=data_portal,
            trading_calendar=trading_calendar,
            sim_params=sim_params,
            metrics_set=metrics_set,
            blotter=blotter,
            benchmark_returns=benchmark_returns,
            benchmark_sid=benchmark_sid,
            handle_data=handle_data,
            analyze=self.analyze,
        )
        return (
            trading_algorithm,
            engine_service_pb2.RunBacktestResponse(
                backtest=backtest_pb2.Backtest(
                    start_date=pb_utils.from_pydate_to_proto_date(trading_algorithm.sim_params.start_session),
                    end_date=pb_utils.from_pydate_to_proto_date(trading_algorithm.sim_params.end_session),
                    symbols=req.backtest.symbols,
                    benchmark=(req.backtest.benchmark if req.backtest.benchmark else None),
                )
            ).SerializeToString(),
        )

    def _place_orders(self, data: bytes, trading_algorithm: TradingAlgorithm) -> bytes:
        req = engine_service_pb2.PlaceOrdersAndContinueRequest()
        req.ParseFromString(data)
        for order in req.orders:
            logging.info("Placing order: %s", order)
            asset = trading_algorithm.symbol(order.symbol)
            order_id = trading_algorithm.order(asset=asset, amount=order.amount)
            logging.debug("Placed order: %s", order_id)

        return engine_service_pb2.PlaceOrdersAndContinueResponse().SerializeToString()

    def _get_current_period(self, data: bytes, trading_algorithm: TradingAlgorithm) -> bytes:
        req = engine_service_pb2.GetCurrentPeriodRequest()
        req.ParseFromString(data)
        p: Portfolio = trading_algorithm.portfolio
        return engine_service_pb2.GetCurrentPeriodResponse(
            is_running=True,
            portfolio=finance_pb2.Portfolio(
                timestamp=pb_utils.to_proto_timestamp(trading_algorithm.datetime),
                cash_flow=p.cash_flow,  # type: ignore
                starting_cash=p.starting_cash,  # type: ignore
                portfolio_value=p.portfolio_value,  # type: ignore
                pnl=p.pnl,  # type: ignore
                returns=p.returns,  # type: ignore
                cash=p.cash,  # type: ignore
                positions_value=p.positions_value,  # type: ignore
                positions_exposure=p.positions_exposure,  # type: ignore
                positions=[
                    finance_pb2.Position(
                        symbol=p.sid.symbol,
                        amount=p.amount,
                        cost_basis=p.cost_basis,
                        last_sale_price=p.last_sale_price,
                        last_sale_date=pb_utils.to_proto_timestamp(p.last_sale_date),
                    )
                    for _, p in p.positions.items()  # type: ignore
                ],
            ),
        ).SerializeToString()

    def _get_execution_result(self, data: bytes) -> bytes:
        req = engine_service_pb2.GetResultRequest()
        req.ParseFromString(data)
        rsp = engine_service_pb2.GetResultResponse()
        for row in self.result.index:
            period = self.result.loc[row]
            rsp.periods.append(
                execution_pb2.Period(
                    date=pb_utils.from_pydate_to_proto_date(period["period_close"].to_pydatetime().replace()),
                    PNL=period["pnl"],
                    returns=period["returns"],
                    portfolio_value=period["portfolio_value"],
                    longs_count=period["longs_count"],
                    shorts_count=period["shorts_count"],
                    long_value=period["long_value"],
                    short_value=period["short_value"],
                    starting_exposure=period["starting_exposure"],
                    ending_exposure=period["ending_exposure"],
                    long_exposure=period["long_exposure"],
                    short_exposure=period["short_exposure"],
                    capital_used=period["capital_used"],
                    gross_leverage=period["gross_leverage"],
                    net_leverage=period["net_leverage"],
                    starting_value=period["starting_value"],
                    ending_value=period["ending_value"],
                    starting_cash=period["starting_cash"],
                    ending_cash=period["ending_cash"],
                    max_drawdown=period["max_drawdown"],
                    max_leverage=period["max_leverage"],
                    excess_return=period["excess_return"],
                    treasury_period_return=period["treasury_period_return"],
                    algorithm_period_return=period["algorithm_period_return"],
                    algo_volatility=(None if pd.isnull(period["algo_volatility"]) else period["algo_volatility"]),
                    sharpe=None if pd.isnull(period["sharpe"]) else period["sharpe"],
                    sortino=None if pd.isnull(period["sortino"]) else period["sortino"],
                    benchmark_period_return=(
                        None if pd.isnull(period["benchmark_period_return"]) else period["benchmark_period_return"]
                    ),
                    benchmark_volatility=(
                        None if pd.isnull(period["benchmark_volatility"]) else period["benchmark_volatility"]
                    ),
                    alpha=(None if period["alpha"] is None or pd.isnull(period["alpha"]) else period["alpha"]),
                    beta=(None if period["beta"] is None or pd.isnull(period["beta"]) else period["beta"]),
                )
            )
        if req.upload:
            storage = Storage.from_environment()
            self.result.to_pick("/tmp/result.pkl")
            storage.upload_object("backtest-results", req.execution, "/tmp/result.pkl")
        return rsp.SerializeToString()

    def _process_request(
        self,
        trading_algorithm: TradingAlgorithm | None,
        bar_data: BarData | None,
        socket: pynng.Socket,
    ):
        while True:
            with socket.new_context() as context_socket:
                req = common_pb2.Request()
                req.ParseFromString(context_socket.recv())
                self.log.debug(f"Received request {req.task}, {id(trading_algorithm)}")
                rsp = common_pb2.Response(task=req.task)
                data: bytes | None = None
                try:
                    match req.task:
                        case "run":
                            if trading_algorithm is not None:
                                rsp.error = "Execution already running"
                                context_socket.send(rsp.SerializeToString())
                            else:
                                ta, data = self._run(req.data, socket)
                                rsp.data = data
                                context_socket.send(rsp.SerializeToString())
                                ta.run()
                                return
                        case "get_current_period":
                            if trading_algorithm is None:
                                rsp.data = engine_service_pb2.GetCurrentPeriodResponse(
                                    is_running=False
                                ).SerializeToString()
                                context_socket.send(rsp.SerializeToString())
                            else:
                                rsp.data = self._get_current_period(req.data, trading_algorithm)
                                context_socket.send(rsp.SerializeToString())
                        case "place_orders_and_continue":
                            if trading_algorithm is None:
                                rsp.error = "Execution not running"
                                context_socket.send(rsp.SerializeToString())
                            else:
                                rsp.data = self._place_orders(req.data, trading_algorithm)
                                context_socket.send(rsp.SerializeToString())
                                return
                        case "stop":
                            context_socket.send(rsp.SerializeToString())
                            raise StopExcecution()
                        case "get_result":
                            rsp.data = self._get_execution_result(req.data)
                            context_socket.send(rsp.SerializeToString())
                except StopExcecution as e:
                    raise e
                except Exception as e:
                    self.log.error(f"Error processing request {req.task}: {e}")
                    rsp.error = str(e)
