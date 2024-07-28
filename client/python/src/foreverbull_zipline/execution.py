import logging
import os
import socket
import tarfile
import threading
import time
from datetime import datetime, timezone

import pandas as pd
import pynng
import pytz
import six
from google.protobuf.timestamp_pb2 import Timestamp
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

from foreverbull.broker.storage import Storage
from foreverbull.entity import backtest
from foreverbull.entity.service import SocketConfig
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2
from foreverbull.pb.service import service_pb2
from foreverbull_zipline.data_bundles.foreverbull import DatabaseEngine, SQLIngester

from . import entity
from .broker import Broker


class ConfigError(Exception):
    pass


class StopExcecution(Exception):
    pass


class Execution(threading.Thread):
    def __init__(self, host=os.getenv("LOCAL_HOST", socket.gethostbyname(socket.gethostname())), port=5555):
        self._socket: pynng.Socket | None = None
        self.socket_config = SocketConfig(
            host=host,
            port=port,
        )
        self._broker: Broker = Broker()
        self._trading_algorithm: TradingAlgorithm | None = None
        self._new_orders = []
        self.logger = logging.getLogger(__name__)
        super(Execution, self).__init__()

    def run(self):
        for _ in range(10):
            try:
                self._socket = pynng.Rep0(listen=f"tcp://{self.socket_config.host}:{self.socket_config.port}")
                break
            except pynng.exceptions.AddressInUse:
                time.sleep(0.1)
        else:
            raise RuntimeError("Could not bind to socket")
        self._socket.recv_timeout = 500
        self._process_request()
        self._socket.close()

    def stop(self):
        socket = pynng.Req0(dial=f"tcp://{self.socket_config.host}:{self.socket_config.port}", block_on_dial=True)
        request = service_pb2.Request(task="stop")
        socket.send(request.SerializeToString())
        socket.recv()
        self.join()
        socket.close()
        return

    def info(self) -> tuple[str, str, SocketConfig]:
        return "backtest", "0.0.0", self.socket_config

    @property
    def ingestion(self) -> tuple[list[str], pd.Timestamp, pd.Timestamp]:
        if self.bundle is None:
            raise LookupError("Bundle is not loaded")
        assets = self.bundle.asset_finder.retrieve_all(self.bundle.asset_finder.sids)
        start = assets[0].start_date.tz_localize("UTC")
        end = assets[0].end_date.tz_localize("UTC")
        return [a.symbol for a in assets], start, end

    def _ingest(self, from_dt: datetime, to_dt: datetime, symbols: list[str]) -> tuple[datetime, datetime, list[str]]:
        self.logger.debug("ingestion started")
        bundles.register("foreverbull", SQLIngester(), calendar_name="XNYS")
        SQLIngester.engine = DatabaseEngine()
        SQLIngester.from_date = from_dt
        SQLIngester.to_date = to_dt
        SQLIngester.symbols = symbols
        bundles.ingest("foreverbull", os.environ, pd.Timestamp.utcnow(), [], True)
        self.bundle: BundleData = bundles.load("foreverbull", os.environ, None)
        self.logger.debug("ingestion completed")
        symbols, start, end = self.ingestion
        return start, end, symbols

    def _download_ingestion(self, name: str):
        storage = Storage.from_environment()
        storage.backtest.download_backtest_ingestion(name, "/tmp/ingestion.tar.gz")
        with tarfile.open("/tmp/ingestion.tar.gz", "r:gz") as tar:
            tar.extractall(data_root())
        bundles.register("foreverbull", SQLIngester())

    def _upload_ingestion(self, name: str):
        with tarfile.open("/tmp/ingestion.tar.gz", "w:gz") as tar:
            tar.add(data_path(["foreverbull"]), arcname="foreverbull")
        storage = Storage.from_environment()
        storage.backtest.upload_backtest_ingestion("/tmp/ingestion.tar.gz", name)

    def _get_algorithm(
        self, start_dt: datetime, end_dt: datetime, symbols: list[str], benchmark: str | None = None
    ) -> TradingAlgorithm:
        # reload, we are in other process
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

        if not symbols:
            symbols = [asset.symbol for asset in bundle.asset_finder.retrieve_all(bundle.asset_finder.sids)]
        else:
            for symbol in symbols:
                asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
                if asset is None:
                    raise ConfigError(f"Unknown symbol: {symbol}")
        try:
            if start_dt:
                start = pd.Timestamp(start_dt)
                if type(start) is not pd.Timestamp:
                    raise ConfigError(f"Invalid start date: {start_dt}")
                start_date = start.normalize().tz_localize(None)
                first_traded_date = find_first_traded_dt(bundle, *symbols)
                if first_traded_date is None:
                    raise ConfigError("unable to determine first traded date")
                if start_date < first_traded_date:
                    start_date = first_traded_date
            else:
                start_date = find_first_traded_dt(bundle, *symbols)
            if not isinstance(start_date, pd.Timestamp):
                raise ConfigError(f"expected start_date to be a pd.Timestamp, is: {type(start_date)}")

            if end_dt:
                end = pd.Timestamp(end_dt)
                if type(end) is not pd.Timestamp:
                    raise ConfigError(f"Invalid end date: {end_dt}")
                end_date = end.normalize().tz_localize(None)
                last_traded_date = find_last_traded_dt(bundle, *symbols)
                if last_traded_date is None:
                    raise ConfigError("unable to determine last traded date")
                if end_date > last_traded_date:
                    end_date = last_traded_date
            else:
                end_date = find_last_traded_dt(bundle, *symbols)
            if not isinstance(end_date, pd.Timestamp):
                raise ConfigError(f"expected end_date to be a pd.Timestamp, is: {type(end_date)}")

        except pytz.exceptions.UnknownTimeZoneError as e:
            self.logger.error("Unknown time zone: %s", repr(e))
            raise ConfigError(repr(e))

        if benchmark:
            benchmark_returns = None
            benchmark_sid = bundle.asset_finder.lookup_symbol(benchmark, as_of_date=None)
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

        trading_algorithm = TradingAlgorithm(
            namespace={"symbols": symbols},
            data_portal=data_portal,
            trading_calendar=trading_calendar,
            sim_params=sim_params,
            metrics_set=metrics_set,
            blotter=blotter,
            benchmark_returns=benchmark_returns,
            benchmark_sid=benchmark_sid,
            handle_data=self._process_request,
            analyze=self.analyze,
        )
        return trading_algorithm

    def analyze(self, _, result):
        self.result = result

    def _upload_result(self, execution: str):
        storage = Storage.from_environment()
        storage.backtest.upload_backtest_result(execution, self.result)

    def _process_request(self, trading_algorithm: TradingAlgorithm | None = None, data: BarData = None):
        while True:
            if self._socket is None:
                return
            context_socket = self._socket.new_context()
            try:
                request = service_pb2.Request()
                request.ParseFromString(context_socket.recv())
                self.logger.info(f"received task: {request.task}")
                try:
                    if request.task == "info":
                        _type, version, socket = self.info()
                        rsp = service_pb2.ServiceInfoResponse(
                            serviceType=_type,
                            version=version,
                            socket=service_pb2.ServiceInfoResponse.Socket(
                                host=socket.host,
                                port=socket.port,
                            ),
                        )
                        response = service_pb2.Response(task=request.task, data=rsp.SerializeToString())
                        context_socket.send(response.SerializeToString())
                    elif request.task == "ingest":
                        req = backtest_pb2.IngestRequest()
                        req.ParseFromString(request.data)
                        start, end, symbols = self._ingest(
                            req.start_date.ToDatetime(), req.end_date.ToDatetime(), [s for s in req.symbols]
                        )
                        rsp = backtest_pb2.IngestResponse(
                            start_date=pb_utils.to_proto_timestamp(start),
                            end_date=pb_utils.to_proto_timestamp(end),
                            symbols=symbols,
                        )
                        response = service_pb2.Response(task=request.task, data=rsp.SerializeToString())
                        context_socket.send(response.SerializeToString())
                    elif request.task == "download_ingestion":
                        self._download_ingestion("foreverbull")
                        response = service_pb2.Response(task=request.task)
                        context_socket.send(response.SerializeToString())
                    elif request.task == "upload_ingestion":
                        self._upload_ingestion("foreverbull")
                        response = service_pb2.Response(task=request.task)
                        context_socket.send(response.SerializeToString())
                    elif request.task == "configure_execution":
                        exc = backtest_pb2.ConfigureRequest()
                        exc.ParseFromString(request.data)
                        self._trading_algorithm = self._get_algorithm(
                            exc.start_date.ToDatetime(),
                            exc.end_date.ToDatetime(),
                            [s for s in exc.symbols],
                        )
                        ce_response = backtest_pb2.ConfigureResponse(
                            start_date=pb_utils.to_proto_timestamp(self._trading_algorithm.sim_params.start_session),
                            end_date=pb_utils.to_proto_timestamp(self._trading_algorithm.sim_params.end_session),
                            symbols=[s for s in self._trading_algorithm.namespace["symbols"]],
                            benchmark=exc.benchmark,
                        )
                        response = service_pb2.Response(task=request.task, data=ce_response.SerializeToString())
                        context_socket.send(response.SerializeToString())
                    elif request.task == "run_execution" and not trading_algorithm:
                        if self._trading_algorithm is None:
                            raise Exception("No execution configured")
                        response = service_pb2.Response(task=request.task)
                        context_socket.send(response.SerializeToString())
                        try:
                            self._trading_algorithm.run()
                        except StopExcecution:
                            pass
                    elif trading_algorithm and data and request.task == "get_portfolio":
                        p: Portfolio = trading_algorithm.portfolio
                        portfolio = backtest_pb2.GetPortfolioResponse(
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
                        )
                        response = service_pb2.Response(task=request.task, data=portfolio.SerializeToString())
                        context_socket.send(response.SerializeToString())
                    elif not trading_algorithm and request.task == "get_portfolio":
                        response = service_pb2.Response(task=request.task, error="no active execution")
                        context_socket.send(response.SerializeToString())
                    elif trading_algorithm and request.task == "continue":
                        req = backtest_pb2.ContinueRequest()
                        req.ParseFromString(request.data)
                        for order in req.orders:
                            self._broker.order(order.symbol, order.amount, trading_algorithm)
                        self._new_orders = []
                        response = service_pb2.Response(task=request.task)
                        context_socket.send(response.SerializeToString())
                        if trading_algorithm:
                            self._new_orders = trading_algorithm.blotter.new_orders
                        return
                    elif not trading_algorithm and request.task == "continue":
                        response = service_pb2.Response(task=request.task, error="no active execution")
                        context_socket.send(response.SerializeToString())
                    elif request.task == "get_execution_result":
                        rsp = backtest_pb2.ResultResponse()
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
                                    algo_volatility=(
                                        None if pd.isnull(period["algo_volatility"]) else period["algo_volatility"]
                                    ),
                                    sharpe=None if pd.isnull(period["sharpe"]) else period["sharpe"],
                                    sortino=None if pd.isnull(period["sortino"]) else period["sortino"],
                                    benchmark_period_return=(
                                        None
                                        if pd.isnull(period["benchmark_period_return"])
                                        else period["benchmark_period_return"]
                                    ),
                                    benchmark_volatility=(
                                        None
                                        if pd.isnull(period["benchmark_volatility"])
                                        else period["benchmark_volatility"]
                                    ),
                                    alpha=(
                                        None
                                        if period["alpha"] is None or pd.isnull(period["alpha"])
                                        else period["alpha"]
                                    ),
                                    beta=(
                                        None if period["beta"] is None or pd.isnull(period["beta"]) else period["beta"]
                                    ),
                                )
                            )
                        response = service_pb2.Response(task=request.task, data=rsp.SerializeToString())
                        context_socket.send(response.SerializeToString())
                    elif request.task == "upload_result":
                        req = backtest_pb2.UploadResultRequest()
                        req.ParseFromString(request.data)
                        self._upload_result(req.execution)
                        response = service_pb2.Response(task=request.task)
                        context_socket.send(response.SerializeToString())
                except Exception as e:
                    self.logger.exception(e)
                    self.logger.error(f"error processing request: {e}")
                    response = service_pb2.Response(task=request.task, error=str(e))
                    context_socket.send(response.SerializeToString())
                    context_socket.close()
                if request.task == "stop" and trading_algorithm:
                    ## Raise to force Zipline TradingAlgorithm to stop, not good way to do this
                    response = service_pb2.Response(task=request.task)
                    context_socket.send(response.SerializeToString())
                    context_socket.close()
                    raise StopExcecution()
                elif request.task == "stop":
                    response = service_pb2.Response(task=request.task)
                    context_socket.send(response.SerializeToString())
                    return
                context_socket.close()
            except pynng.Timeout:
                self.logger.debug("timeout")
                context_socket.close()
