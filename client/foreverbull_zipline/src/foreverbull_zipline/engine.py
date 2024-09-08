import functools
import logging
import logging.handlers
import multiprocessing
import multiprocessing.queues
import os
import tarfile
from abc import ABC, abstractmethod
from datetime import timezone

import pandas as pd
import pynng
import pytz
import six
from foreverbull.broker.storage import Storage
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2
from foreverbull.pb.service import service_pb2
from foreverbull_zipline.data_bundles.foreverbull import DatabaseEngine, SQLIngester
from zipline import TradingAlgorithm
from zipline.data import bundles
from zipline.data.bundles.core import BundleData
from zipline.data.data_portal import DataPortal
from zipline.extensions import load
from zipline.finance import metrics
from zipline.finance.blotter import Blotter
from zipline.finance.trading import SimulationParameters
from zipline.protocol import BarData, Portfolio
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.paths import data_path, data_root


class ConfigError(Exception):
    pass


class StopExcecution(Exception):
    pass


class Engine(ABC):
    @abstractmethod
    def ingest(self, ingestion: backtest_pb2.IngestRequest) -> backtest_pb2.IngestResponse:
        pass

    @abstractmethod
    def run_backtest(self, backtest: backtest_pb2.RunRequest) -> backtest_pb2.RunResponse:
        pass

    @abstractmethod
    def place_orders(self, req: backtest_pb2.PlaceOrdersRequest) -> backtest_pb2.PlaceOrdersResponse:
        pass

    @abstractmethod
    def get_next_period(self, req: backtest_pb2.GetNextPeriodRequest) -> backtest_pb2.GetNextPeriodResponse:
        pass

    @abstractmethod
    def get_backtest_result(self, req: backtest_pb2.GetResultRequest) -> backtest_pb2.GetResultResponse:
        pass

    @abstractmethod
    def stop(self):
        pass


class EngineProcess(multiprocessing.Process, Engine):
    def __init__(
        self,
        socket_file_path: str = "/tmp/foreverbull_zipline.sock",
        logging_queue: multiprocessing.queues.Queue | None = None,
        download_ingestion: bool = False,
    ):
        self._socket_file_path = socket_file_path
        self._logging_queue = logging_queue
        self._download_ingestion = download_ingestion
        self.is_ready = multiprocessing.Event()
        super(EngineProcess, self).__init__()

    def ingest(self, ingestion: backtest_pb2.IngestRequest) -> backtest_pb2.IngestResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}", block_on_dial=True, recv_timeout=10_000, send_timeout=10_000
        ) as socket:
            bytes = ingestion.SerializeToString()
            request = service_pb2.Request(task="ingest", data=bytes)
            socket.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            ing = backtest_pb2.IngestResponse()
            ing.ParseFromString(response.data)
            return ing

    def run_backtest(self, backtest: backtest_pb2.RunRequest) -> backtest_pb2.RunResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}", block_on_dial=True, recv_timeout=10_000, send_timeout=10_000
        ) as socket:
            data = backtest.SerializeToString()
            request = service_pb2.Request(task="run", data=data)
            socket.send(request.SerializeToString())
            response = service_pb2.Response()
            if response.HasField("error"):
                raise SystemError(response.error)
            response.ParseFromString(socket.recv())
            b = backtest_pb2.RunResponse()
            b.ParseFromString(response.data)
            return b

    def place_orders(self, req: backtest_pb2.PlaceOrdersRequest) -> backtest_pb2.PlaceOrdersResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}", block_on_dial=True, recv_timeout=10_000, send_timeout=10_000
        ) as socket:
            data = req.SerializeToString()
            request = service_pb2.Request(task="place_orders", data=data)
            socket.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            p = backtest_pb2.PlaceOrdersResponse()
            p.ParseFromString(response.data)
            return p

    def get_next_period(self, req: backtest_pb2.GetNextPeriodRequest) -> backtest_pb2.GetNextPeriodResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}", block_on_dial=True, recv_timeout=10_000, send_timeout=10_000
        ) as socket:
            data = req.SerializeToString()
            request = service_pb2.Request(task="get_next_period", data=data)
            socket.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            p = backtest_pb2.GetNextPeriodResponse()
            p.ParseFromString(response.data)
            return p

    def get_backtest_result(self, req: backtest_pb2.GetResultRequest) -> backtest_pb2.GetResultResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}", block_on_dial=True, recv_timeout=10_000, send_timeout=10_000
        ) as socket:
            request = service_pb2.Request(task="get_result")
            socket.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(socket.recv())
            if response.HasField("error"):
                raise SystemError(response.error)
            result = backtest_pb2.GetResultResponse()
            result.ParseFromString(response.data)
            return result

    def stop(self):
        try:
            with pynng.Req0(
                dial=f"ipc://{self._socket_file_path}", block_on_dial=True, recv_timeout=1_000, send_timeout=1_000
            ) as socket:
                request = service_pb2.Request(task="stop")
                try:
                    socket.send(request.SerializeToString())
                    socket.recv()
                except pynng.Timeout:
                    print("TIMEOUT")
                    pass
        except pynng.exceptions.ConnectionRefused:
            print("REFUESED")
            pass

    def run(self):
        if self._download_ingestion:
            storage = Storage.from_environment()
            storage.backtest.download_backtest_ingestion("foreverbull", "/tmp/ingestion.tar.gz")
            with tarfile.open("/tmp/ingestion.tar.gz", "r:gz") as tar:
                tar.extractall(data_root())
            bundles.register("foreverbull", SQLIngester())
        if self._logging_queue is not None:
            handler = logging.handlers.QueueHandler(self._logging_queue)
            logging.basicConfig(handlers=[handler], level=logging.DEBUG)
        self.log = logging.getLogger(__name__)
        self.log.info("Starting Execution Process")
        self._trading_algorithm: TradingAlgorithm | None = None
        self.logger = logging.getLogger(__name__)
        import os

        if os.path.exists(self._socket_file_path):
            os.remove(self._socket_file_path)
        with pynng.Rep0(listen=f"ipc://{self._socket_file_path}", block_on_dial=True, send_timeout=1_000) as socket:
            self.is_ready.set()
            while True:
                try:
                    self._process_request(None, None, socket)
                except (StopExcecution, KeyboardInterrupt):
                    break

    @property
    def ingestion(self) -> tuple[list[str], pd.Timestamp, pd.Timestamp]:
        if self.bundle is None:
            raise LookupError("Bundle is not loaded")
        assets = self.bundle.asset_finder.retrieve_all(self.bundle.asset_finder.sids)
        start = assets[0].start_date.tz_localize("UTC")
        end = assets[0].end_date.tz_localize("UTC")
        return [a.symbol for a in assets], start, end

    def analyze(self, _, result):
        self.result = result

    def _ingest(self, data: bytes) -> bytes:
        req = backtest_pb2.IngestRequest()
        req.ParseFromString(data)
        bundles.register("foreverbull", SQLIngester(), calendar_name="XNYS")
        SQLIngester.engine = DatabaseEngine()
        SQLIngester.from_date = req.ingestion.start_date.ToDatetime()
        SQLIngester.to_date = req.ingestion.end_date.ToDatetime()
        SQLIngester.symbols = [s for s in req.ingestion.symbols]
        bundles.ingest("foreverbull", os.environ, pd.Timestamp.utcnow(), [], True)
        self.bundle: BundleData = bundles.load("foreverbull", os.environ, None)
        self.logger.debug("ingestion completed")
        symbols, start, end = self.ingestion
        if req.upload:
            with tarfile.open("/tmp/ingestion.tar.gz", "w:gz") as tar:
                tar.add(data_path(["foreverbull"]), arcname="foreverbull")
            storage = Storage.from_environment()
            storage.backtest.upload_backtest_ingestion("/tmp/ingestion.tar.gz", "foreverbull")
        return backtest_pb2.IngestResponse(
            ingestion=backtest_pb2.Ingestion(
                start_date=pb_utils.to_proto_timestamp(start),
                end_date=pb_utils.to_proto_timestamp(end),
                symbols=symbols,
            )
        ).SerializeToString()

    def _run(self, data: bytes, socket: pynng.Socket) -> tuple[TradingAlgorithm, bytes]:
        req = backtest_pb2.RunRequest()
        req.ParseFromString(data)
        bundle = bundles.load("foreverbull", os.environ, None)

        def find_last_traded_dt(bundle: BundleData, *symbols):
            last_traded = None
            for symbol in symbols:
                asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
                if asset is None:
                    continue
                if last_traded is None:
                    last_traded = asset.end_date
                else:
                    last_traded = max(last_traded, asset.end_date)
            return last_traded

        def find_first_traded_dt(bundle: BundleData, *symbols):
            first_traded = None
            for symbol in symbols:
                asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
                if asset is None:
                    continue
                if first_traded is None:
                    first_traded = asset.start_date
                else:
                    first_traded = min(first_traded, asset.start_date)
            return first_traded

        for symbol in req.backtest.symbols:
            asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
            if asset is None:
                raise ConfigError(f"Unknown symbol: {symbol}")
        try:
            if req.backtest.start_date:
                start = pd.Timestamp(req.backtest.start_date.ToDatetime())
                if type(start) is not pd.Timestamp:
                    raise ConfigError(f"Invalid start date: {req.backtest.start_date.ToDatetime()}")
                start_date = start.normalize().tz_localize(None)
                first_traded_date = find_first_traded_dt(bundle, *req.backtest.symbols)
                if first_traded_date is None:
                    raise ConfigError("unable to determine first traded date")
                if start_date < first_traded_date:
                    start_date = first_traded_date
            else:
                start_date = find_first_traded_dt(bundle, *req.backtest.symbols)
            if not isinstance(start_date, pd.Timestamp):
                raise ConfigError(f"expected start_date to be a pd.Timestamp, is: {type(start_date)}")

            if req.backtest.end_date:
                end = pd.Timestamp(req.backtest.end_date.ToDatetime())
                if type(end) is not pd.Timestamp:
                    raise ConfigError(f"Invalid end date: {pd.Timestamp(req.backtest.end_date.ToDatetime())}")
                end_date = end.normalize().tz_localize(None)
                last_traded_date = find_last_traded_dt(bundle, *req.backtest.symbols)
                if last_traded_date is None:
                    raise ConfigError("unable to determine last traded date")
                if end_date > last_traded_date:
                    end_date = last_traded_date
            else:
                end_date = find_last_traded_dt(bundle, *req.backtest.symbols)
            if not isinstance(end_date, pd.Timestamp):
                raise ConfigError(f"expected end_date to be a pd.Timestamp, is: {type(end_date)}")

        except pytz.exceptions.UnknownTimeZoneError as e:
            self.logger.error("Unknown time zone: %s", repr(e))
            raise ConfigError(repr(e))

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
            backtest_pb2.RunResponse(
                backtest=backtest_pb2.Backtest(
                    start_date=pb_utils.to_proto_timestamp(trading_algorithm.sim_params.start_session),
                    end_date=pb_utils.to_proto_timestamp(trading_algorithm.sim_params.end_session),
                    symbols=req.backtest.symbols,
                    benchmark=req.backtest.benchmark if req.backtest.benchmark else None,
                )
            ).SerializeToString(),
        )

    def _place_orders(self, data: bytes, trading_algorithm: TradingAlgorithm) -> bytes:
        req = backtest_pb2.PlaceOrdersRequest()
        req.ParseFromString(data)
        for order in req.orders:
            asset = trading_algorithm.symbol(order.symbol)
            trading_algorithm.order(asset=asset, amount=order.amount)
        return backtest_pb2.PlaceOrdersResponse().SerializeToString()

    def _get_next_period(self, data: bytes, trading_algorithm: TradingAlgorithm) -> bytes:
        req = backtest_pb2.GetNextPeriodRequest()
        req.ParseFromString(data)
        p: Portfolio = trading_algorithm.portfolio
        return backtest_pb2.GetNextPeriodResponse(
            is_running=True,
            portfolio=backtest_pb2.Portfolio(
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
                    backtest_pb2.Position(
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
        req = backtest_pb2.GetResultRequest()
        req.ParseFromString(data)
        rsp = backtest_pb2.GetResultResponse()
        for row in self.result.index:
            period = self.result.loc[row]
            rsp.periods.append(
                backtest_pb2.Period(
                    timestamp=pb_utils.to_proto_timestamp(
                        period["period_close"].to_pydatetime().replace(tzinfo=timezone.utc)
                    ),
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
            storage.backtest.upload_backtest_result(req.execution, self.result)
        return rsp.SerializeToString()

    def _process_request(
        self, trading_algorithm: TradingAlgorithm | None, bar_data: BarData | None, socket: pynng.Socket
    ):
        with socket.new_context() as context_socket:
            req = service_pb2.Request()
            req.ParseFromString(context_socket.recv())
            self.log.debug(f"Received request {req.task}")
            rsp = service_pb2.Response(task=req.task)
            data: bytes | None = None
            try:
                if trading_algorithm:
                    match req.task:
                        case "place_orders":
                            data = self._place_orders(req.data, trading_algorithm)
                        case "get_next_period":
                            data = self._get_next_period(req.data, trading_algorithm)
                        case "stop":
                            context_socket.send(rsp.SerializeToString())
                            raise StopExcecution()
                        case _:
                            raise Exception(f"Unknown task {req.task}")
                else:
                    match req.task:
                        case "ingest":
                            data = self._ingest(req.data)
                        case "run":
                            ta, data = self._run(req.data, socket)
                            response = service_pb2.Response(task=req.task, data=data)
                            context_socket.send(response.SerializeToString())
                            ta.run()
                            return
                        case "get_result":
                            data = self._get_execution_result(req.data)
                        case "get_next_period":
                            data = backtest_pb2.GetNextPeriodResponse(is_running=False).SerializeToString()
                        case "stop":
                            print("STOP REQ", flush=True)
                            pass
                        case _:
                            raise Exception(f"Unknown task {req.task}")
            except StopExcecution as e:
                raise e
            except Exception as e:
                self.log.error(f"Error processing request {req.task}: {e}")
                rsp.error = str(e)
            rsp.task = req.task
            if data:
                rsp.data = data
            context_socket.send(rsp.SerializeToString())
            if req.task == "stop":
                print("RAISE STOP", flush=True)
                raise StopExcecution()
