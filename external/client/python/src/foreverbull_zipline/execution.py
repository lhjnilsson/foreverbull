import logging
import os
import socket
import tarfile
import threading

import pandas as pd
import pynng
import pytz
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
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.paths import data_path, data_root

from foreverbull.broker.storage import Storage
from foreverbull.entity import backtest
from foreverbull.entity.service import Request, Response, SocketConfig
from foreverbull_zipline.data_bundles.foreverbull import DatabaseEngine, SQLIngester

from . import entity
from .broker import Broker


class ConfigError(Exception):
    pass


class StopExcecution(Exception):
    pass


class Execution(threading.Thread):
    def __init__(self, host=os.getenv("LOCAL_HOST", socket.gethostbyname(socket.gethostname())), port=5555):
        self._socket = None
        self.socket_config = SocketConfig(
            host=host,
            port=port,
        )
        self._broker: Broker = None
        self._trading_algorithm: TradingAlgorithm = None
        self._new_orders = []
        self.logger = logging.getLogger(__name__)
        super(Execution, self).__init__()

    def run(self):
        self._socket = pynng.Rep0(listen=f"tcp://{self.socket_config.host}:{self.socket_config.port}")
        self._socket.recv_timeout = 500
        self._broker = Broker()
        self._process_request()
        self._socket.close()

    def stop(self):
        socket = pynng.Req0(dial=f"tcp://{self.socket_config.host}:{self.socket_config.port}", block_on_dial=True)
        request = Request(task="stop")
        socket.send(request.dump())
        socket.recv()
        self.join()
        socket.close()
        return

    def info(self):
        return {
            "type": "backtest",
            "version": "0.0.0",
            "socket": self.socket_config.model_dump(),
        }

    @property
    def ingestion(self) -> entity.IngestConfig:
        if self.bundle is None:
            return entity.IngestConfig(calendar=None, start=None, end=None, symbols=[])
        assets = self.bundle.asset_finder.retrieve_all(self.bundle.asset_finder.sids)
        start = assets[0].start_date.tz_localize("UTC")
        end = assets[0].end_date.tz_localize("UTC")
        calendar = self.bundle.equity_daily_bar_reader.trading_calendar.name
        return entity.IngestConfig(calendar=calendar, start=start, end=end, symbols=[asset.symbol for asset in assets])

    def _ingest(self, config: entity.IngestConfig) -> entity.IngestConfig:
        self.logger.debug("ingestion started")
        bundles.register("foreverbull", SQLIngester(), calendar_name=config.calendar)
        SQLIngester.engine = DatabaseEngine()
        SQLIngester.from_date = config.start
        SQLIngester.to_date = config.end
        SQLIngester.symbols = config.symbols
        bundles.ingest("foreverbull", os.environ, pd.Timestamp.utcnow(), [], True)
        self.bundle: BundleData = bundles.load("foreverbull", os.environ, None)
        self.logger.debug("ingestion completed")
        return self.ingestion

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

    def _get_algorithm(self, config: backtest.Execution):
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

        if config.symbols is None:
            symbols = [asset.symbol for asset in bundle.asset_finder.retrieve_all(bundle.asset_finder.sids)]
        else:
            symbols = []
            for symbol in config.symbols:
                asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
                if asset is None:
                    raise ConfigError(f"Unknown symbol: {symbol}")
                symbols.append(asset.symbol)

        try:
            if config.start:
                start_date = pd.Timestamp(config.start).normalize().tz_localize(None)
                first_traded_date = find_first_traded_dt(bundle, *symbols)
                if start_date < first_traded_date:
                    start_date = first_traded_date
            else:
                start_date = find_first_traded_dt(bundle, *symbols)

            if config.end:
                end_date = pd.Timestamp(config.end).normalize().tz_localize(None)
                last_traded_date = find_last_traded_dt(bundle, *symbols)
                if end_date > last_traded_date:
                    end_date = last_traded_date
            else:
                end_date = find_last_traded_dt(bundle, *symbols)
        except pytz.exceptions.UnknownTimeZoneError as e:
            self.logger.error("Unknown time zone: %s", repr(e))
            raise ConfigError(repr(e))

        if config.benchmark:
            benchmark_returns = None
            benchmark_sid = bundle.asset_finder.lookup_symbol(config.benchmark, as_of_date=None)
        else:
            benchmark_returns = pd.Series(index=pd.date_range(start_date, end_date, tz="utc"), data=0.0)
            benchmark_sid = None

        trading_calendar = get_calendar(config.calendar)
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
            capital_base=config.capital_base,
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

        config.calendar = trading_calendar.name
        config.start = start_date.to_pydatetime()
        config.end = end_date.to_pydatetime()
        config.benchmark = benchmark_sid.symbol if benchmark_sid else None
        config.symbols = symbols
        return trading_algorithm, config

    def analyze(self, _, result):
        self.result = result

    def _result(self):
        return entity.Result.from_zipline(self.result)

    def _upload_result(self, execution: str):
        storage = Storage.from_environment()
        storage.backtest.upload_backtest_result(execution, self.result)

    def _process_request(self, trading_algorithm: TradingAlgorithm = None, data: BarData = None):
        while True:
            try:
                context_socket = self._socket.new_context()
                message = Request.load(context_socket.recv())
                self.logger.info(f"received task: {message.task}")
                active_execution = trading_algorithm and data
                try:
                    if message.task == "info":
                        context_socket.send(Response(task=message.task, data=self.info()).dump())
                    elif message.task == "ingest":
                        ingest_config = entity.IngestConfig(**message.data)
                        ingest_config = self._ingest(ingest_config)
                        context_socket.send(Response(task=message.task, data=ingest_config).dump())
                    elif message.task == "download_ingestion":
                        self._download_ingestion(**message.data)
                        context_socket.send(Response(task=message.task).dump())
                    elif message.task == "upload_ingestion":
                        self._upload_ingestion(**message.data)
                        context_socket.send(Response(task=message.task).dump())
                    elif message.task == "configure_execution":
                        config = backtest.Execution(**message.data)
                        self._trading_algorithm, config = self._get_algorithm(config)
                        context_socket.send(Response(task=message.task, data=config).dump())
                    elif message.task == "run_execution" and not active_execution:
                        context_socket.send(Response(task=message.task).dump())
                        try:
                            self._trading_algorithm.run()
                        except StopExcecution:
                            pass
                    elif active_execution and message.task == "get_period":
                        period = entity.RunningPeriod.from_zipline(
                            trading_algorithm, [trading_algorithm.get_order(order.id) for order in self._new_orders]
                        )
                        context_socket.send(Request(task=message.task, data=period).dump())
                    elif not active_execution and message.task == "get_period":
                        context_socket.send(Response(task=message.task, data=None).dump())
                    elif active_execution and message.task == "continue":
                        self._new_orders = []
                        context_socket.send(Response(task=message.task).dump())
                        if trading_algorithm:
                            self._new_orders = trading_algorithm.blotter.new_orders
                        return
                    elif not active_execution and message.task == "continue":
                        context_socket.send(Response(task=message.task, error="no active execution").dump())
                    elif active_execution and message.task == "can_trade":
                        asset = entity.Asset(**message.data)
                        can_trade = self._broker.can_trade(asset, trading_algorithm, data)
                        context_socket.send(Response(task=message.task, data=can_trade).dump())
                    elif active_execution and message.task == "order":
                        order = entity.Order(**message.data)
                        order = self._broker.order(order, trading_algorithm)
                        context_socket.send(Response(task=message.task, data=order).dump())
                    elif active_execution and message.task == "get_order":
                        order = entity.Order(**message.data)
                        order = self._broker.get_order(order, trading_algorithm)
                        context_socket.send(Response(task=message.task, data=order).dump())
                    elif active_execution and message.task == "get_open_orders":
                        orders = self._broker.get_open_orders(trading_algorithm)
                        context_socket.send(Response(task=message.task, data=orders).dump())
                    elif active_execution and message.task == "cancel_order":
                        order = entity.Order(**message.data)
                        order = self._broker.cancel_order(order, trading_algorithm)
                        context_socket.send(Response(task=message.task, data=order).dump())
                    elif message.task == "get_execution_result":
                        result = self._result()
                        context_socket.send(Response(task=message.task, data=result.model_dump()).dump())
                    elif message.task == "upload_result":
                        self._upload_result(**message.data)
                        context_socket.send(Response(task=message.task).dump())
                except Exception as e:
                    self.logger.exception(e)
                    self.logger.error(f"error processing request: {e}")
                    context_socket.send(Response(task=message.task, error=str(e)).dump())
                    context_socket.close()
                if message.task == "stop" and active_execution:
                    ## Raise to force Zipline TradingAlgorithm to stop, not good way to do this
                    context_socket.send(Response(task=message.task).dump())
                    context_socket.close()
                    raise StopExcecution()
                elif message.task == "stop":
                    context_socket.send(Response(task=message.task).dump())
                    return
                context_socket.close()
            except pynng.Timeout:
                self.logger.debug("timeout")
                context_socket.close()
