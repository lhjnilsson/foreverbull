import functools
import logging
import logging.handlers
import multiprocessing
import multiprocessing.queues
import os

from typing import cast

import numpy as np
import pandas as pd
import pynng
import six

from protobuf import merge_from_binary
from zipline.data import bundles
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
from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb as engine_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull_zipline.data_bundles.bundle_alignment import ensure_session_aligned_bundle
from foreverbull_zipline.errors import PlainSentence
from zipline import TradingAlgorithm


class ConfigError(Exception):
    pass


class StopExcecution(Exception):
    pass


# The engine process puts this before a PlainSentence's message in
# Response.error, so the parent raises it as a PlainSentence again and not
# as an internal failure.
PLAIN_SENTENCE_TAG = "plain-sentence: "


def raise_response_error(response: common_pb.Response) -> None:
    """Raise the engine process's error, a PlainSentence when it sent one."""
    if not response.has_field("error"):
        return
    error = response.error or ""
    if error.startswith(PLAIN_SENTENCE_TAG):
        raise PlainSentence(error.removeprefix(PLAIN_SENTENCE_TAG))
    raise SystemError(error)


def zero_benchmark_returns(sessions: pd.DatetimeIndex) -> pd.Series:
    """Flat benchmark on the simulation sessions, not on calendar dates."""
    return pd.Series(index=sessions, data=0.0)


def benchmark_returns(window_sessions, sessions, opens, closes) -> pd.Series:
    """Benchmark daily returns on Zipline session labels.

    ``window_sessions`` is ``sessions``, or ``sessions`` with the session
    before it, and every open and close in it is a stored price above 0
    (``load_benchmark_returns`` fails the run otherwise), so no return is
    filled. With the session before, the first return is the first close
    against that close and the window day itself is not returned: Zipline
    only reads the run sessions. Without it, the run starts on the
    benchmark's first bundle session, where no previous close exists, and
    the first return is that session's open to close, as Zipline does for
    an asset that starts on that session. Passing the benchmark as a sid
    instead lets Zipline compare the asset start (the first bar, stamped at
    the session open) against the session label and refuse the run, so the
    series is built here.
    """
    session_index = pd.DatetimeIndex(sessions)
    if len(session_index) == 0:
        return zero_benchmark_returns(session_index)
    window_index = pd.DatetimeIndex(window_sessions)
    close_series = pd.Series(data=[float(value) for value in closes], index=window_index, dtype=float)
    open_series = pd.Series(data=[float(value) for value in opens], index=window_index, dtype=float)
    returns = close_series / close_series.shift(1) - 1.0
    if window_index[0] < session_index[0]:
        # The window day only lends its close; its own return has no
        # previous close and Zipline doesn't read it.
        returns = returns.iloc[1:]
    else:
        first = session_index[0]
        returns.iloc[0] = (close_series[first] - open_series[first]) / open_series[first]
    return returns.loc[session_index]


def load_benchmark_returns(backtest: str, bundle, benchmark_asset, trading_calendar, sessions) -> pd.Series:
    """Read the benchmark's daily bars from the bundle on session labels.

    The window starts one session early so the first return is measured
    from the previous close. When the run starts on the benchmark's first
    session (its asset start date) there is no previous close, and the
    first return is that session's open to close. A window day the bundle has no bar for, or
    whose open or close is missing or not above 0, fails the run and every
    such day is named. ``reader.sessions`` is the bundle calendar, not the
    days the benchmark has bars, and bcolz reads a stored 0 as NaN, so the
    prices themselves are checked.
    """
    if len(sessions) == 0:
        return zero_benchmark_returns(sessions)
    reader = bundle.equity_daily_bar_reader
    bundle_sessions = pd.DatetimeIndex(reader.sessions)
    first_session = benchmark_first_session(benchmark_asset)
    window_start = sessions[0]
    if sessions[0] > first_session:
        window_start = trading_calendar.previous_session(sessions[0])
    window_sessions = pd.DatetimeIndex(trading_calendar.sessions_in_range(window_start, sessions[-1]))
    opens = np.full(len(window_sessions), np.nan)
    closes = np.full(len(window_sessions), np.nan)
    readable = (window_sessions >= bundle_sessions[0]) & (window_sessions <= bundle_sessions[-1])
    if readable.any():
        stored = window_sessions[readable]
        stored_opens, stored_closes = reader.load_raw_arrays(
            ["open", "close"], stored[0], stored[-1], [benchmark_asset]
        )
        opens[readable] = stored_opens[:, 0]
        closes[readable] = stored_closes[:, 0]
    usable = np.isfinite(opens) & np.isfinite(closes) & (opens > 0) & (closes > 0)
    usable &= window_sessions.isin(bundle_sessions)
    if not usable.all():
        missing = pd.DatetimeIndex(window_sessions[~usable])
        raise PlainSentence(benchmark_missing_line(backtest, str(benchmark_asset.symbol), missing, window_sessions))
    return benchmark_returns(window_sessions, sessions, opens, closes)


def benchmark_first_session(benchmark_asset) -> pd.Timestamp:
    """The session label of the benchmark's first stored bar."""
    first = cast(pd.Timestamp, pd.Timestamp(benchmark_asset.start_date))
    if first.tzinfo is not None:
        first = first.tz_convert(None)
    return first.normalize()


def session_ranges(days: pd.DatetimeIndex, sessions: pd.DatetimeIndex) -> str:
    """Days as dates, and a run of sessions in a row as "A to B".

    A weekend or holiday inside a run doesn't break it, as in the Go
    side's missing-session list.
    """
    positions = [int(position) for position in sessions.get_indexer(days)]
    parts: list[str] = []
    i = 0
    while i < len(positions):
        j = i
        while j + 1 < len(positions) and positions[j + 1] == positions[j] + 1:
            j += 1
        first = f"{days[i]:%Y-%m-%d}"
        parts.append(first if j == i else f"{first} to {days[j]:%Y-%m-%d}")
        i = j + 1
    return ", ".join(parts)


def benchmark_missing_line(backtest: str, benchmark: str, missing: pd.DatetimeIndex, sessions: pd.DatetimeIndex) -> str:
    """Name the backtest, the benchmark and every session it has no price on."""
    count = "1 trading day" if len(missing) == 1 else f"{len(missing)} trading days"
    return (
        f"Couldn't run backtest '{backtest}': the benchmark {benchmark} has no prices on {count}: "
        f"{session_ranges(missing, sessions)}. Nothing was run. "
        "Run `fbull backtest create` again to reload its prices, or choose another benchmark."
    )


def ticker_history(asset_finder) -> dict[int, list[tuple[pd.Timestamp, str]]]:
    """Each sid's tickers by the session they start on, oldest first."""
    history: dict[int, list[tuple[pd.Timestamp, str]]] = {}
    for owners in asset_finder.symbol_ownership_map.values():
        for owner in owners:
            history.setdefault(int(owner.sid), []).append((_session_day(owner.start), str(owner.value)))
    for tickers in history.values():
        tickers.sort()
    return history


def ticker_on(tickers: list[tuple[pd.Timestamp, str]], day: pd.Timestamp) -> str:
    """The ticker that started last on or before day.

    Zipline names an asset by its latest ticker, so a position held
    before a rename (FB then META) would read META while the universe
    still lists FB.
    """
    current = tickers[0][1]
    for start, symbol in tickers:
        if start > day:
            break
        current = symbol
    return current


def _session_day(value) -> pd.Timestamp:
    ts = cast(pd.Timestamp, pd.Timestamp(value))
    if ts.tzinfo is not None:
        ts = ts.tz_convert("UTC").tz_localize(None)
    return ts.normalize()


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
        super().__init__()

    def run_backtest(self, backtest: engine_service_pb.RunBacktestRequest) -> engine_service_pb.RunBacktestResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            data = backtest.to_binary()
            request = common_pb.Request(task="run", data=data)
            socket.send(request.to_binary())
            response = common_pb.Response()
            merge_from_binary(response, socket.recv())
            raise_response_error(response)
            b = engine_service_pb.RunBacktestResponse()
            merge_from_binary(b, response.data)
            return b

    def get_current_period(
        self, req: engine_service_pb.GetCurrentPeriodRequest
    ) -> engine_service_pb.GetCurrentPeriodResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            data = req.to_binary()
            request = common_pb.Request(task="get_current_period", data=data)
            socket.send(request.to_binary())
            response = common_pb.Response()
            merge_from_binary(response, socket.recv())
            raise_response_error(response)
            p = engine_service_pb.GetCurrentPeriodResponse()
            merge_from_binary(p, response.data)
            return p

    def place_orders_and_continue(
        self, req: engine_service_pb.PlaceOrdersAndContinueRequest
    ) -> engine_service_pb.PlaceOrdersAndContinueResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            data = req.to_binary()
            request = common_pb.Request(task="place_orders_and_continue", data=data)
            socket.send(request.to_binary())
            response = common_pb.Response()
            merge_from_binary(response, socket.recv())
            raise_response_error(response)
            p = engine_service_pb.PlaceOrdersAndContinueResponse()
            merge_from_binary(p, response.data)
            return p

    def get_backtest_result(self, req: engine_service_pb.GetResultRequest) -> engine_service_pb.GetResultResponse:
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            request = common_pb.Request(task="get_result", data=req.to_binary())
            socket.send(request.to_binary())
            response = common_pb.Response()
            merge_from_binary(response, socket.recv())
            raise_response_error(response)
            result = engine_service_pb.GetResultResponse()
            merge_from_binary(result, response.data)
            return result

    def stop(self):
        with pynng.Req0(
            dial=f"ipc://{self._socket_file_path}",
            block_on_dial=False,
            recv_timeout=10_000,
            send_timeout=10_000,
        ) as socket:
            request = common_pb.Request(task="stop")
            socket.send(request.to_binary())

    def run(self):
        if self._logging_queue is not None:
            handler = logging.handlers.QueueHandler(self._logging_queue)
            logging.basicConfig(handlers=[handler], level=logging.DEBUG)
        self.log = logging.getLogger().getChild(__name__)
        self.log.info("Starting Execution Process")
        self._trading_algorithm: TradingAlgorithm | None = None
        self.logger = logging.getLogger().getChild(__name__)

        if os.path.exists(self._socket_file_path):
            print("Removing old socket file", self._socket_file_path)
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
        req = engine_service_pb.RunBacktestRequest()
        merge_from_binary(req, data)
        execution = req.execution
        if execution is None:
            raise ValueError("missing execution")
        start = execution.start_date
        end = execution.end_date
        if start is None or end is None:
            raise ValueError("missing execution dates")
        bundles.register(execution.backtest, None, calendar_name="XNYS")
        # A bundle ingested before the session-alignment fix still carries
        # 14:30 asset dates; rebuild it from its own bars once, here on the
        # run path, so a returning user is not stuck with the old bundle.
        # Current bundles are gated by the version marker and not rebuilt.
        ensure_session_aligned_bundle(execution.backtest, os.environ)
        bundle = bundles.load(execution.backtest, os.environ, None)

        start_date = pd.Timestamp(
            start.year,
            start.month,
            start.day,
        )
        end_date = pd.Timestamp(
            end.year,
            end.month,
            end.day,
        )

        trading_calendar = get_calendar("XNYS")
        sessions = trading_calendar.sessions_in_range(start_date, end_date)  # type: ignore[arg-type]

        benchmark_sid = None
        if req.benchmark:
            benchmark_asset = bundle.asset_finder.lookup_symbol(req.benchmark, as_of_date=None)
            stored_benchmark_returns = load_benchmark_returns(
                execution.backtest, bundle, benchmark_asset, trading_calendar, sessions
            )
        else:
            stored_benchmark_returns = zero_benchmark_returns(sessions)

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
        num_periods = int(sessions.size)  # type: ignore
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
            benchmark_returns=stored_benchmark_returns,
            benchmark_sid=benchmark_sid,
            handle_data=handle_data,
            analyze=self.analyze,
        )
        return (
            trading_algorithm,
            engine_service_pb.RunBacktestResponse(
                execution=req.execution,
                num_periods=num_periods,
            ).to_binary(),
        )

    def _place_orders(self, data: bytes, trading_algorithm: TradingAlgorithm) -> bytes:
        req = engine_service_pb.PlaceOrdersAndContinueRequest()
        merge_from_binary(req, data)
        for order in req.orders:
            self.log.info("Placing order: %s", order)
            # TradingAlgorithm.symbol resolves as of the last session. A
            # ticker reused by another ISIN later in the backtest must still
            # map to the ISIN that held it on this session.
            asset = trading_algorithm.asset_finder.lookup_symbol(
                order.symbol,
                as_of_date=_session_day(trading_algorithm.datetime),
            )
            order_id = trading_algorithm.order(asset=asset, amount=order.amount)
            self.log.debug("Placed order: %s", order_id)

        return engine_service_pb.PlaceOrdersAndContinueResponse().to_binary()

    def _get_current_period(self, data: bytes, trading_algorithm: TradingAlgorithm) -> bytes:
        req = engine_service_pb.GetCurrentPeriodRequest()
        merge_from_binary(req, data)
        p: Portfolio = trading_algorithm.portfolio
        return engine_service_pb.GetCurrentPeriodResponse(
            is_running=True,
            portfolio=marketdata_pb.Portfolio(
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
                    marketdata_pb.Position(
                        symbol=self._position_symbol(trading_algorithm, p.sid),
                        amount=int(p.amount),
                        cost_basis=p.cost_basis,
                        last_sale_price=p.last_sale_price,
                        last_sale_date=pb_utils.to_proto_timestamp(p.last_sale_date),
                    )
                    for _, p in p.positions.items()  # type: ignore
                ],
            ),
        ).to_binary()

    def _position_symbol(self, trading_algorithm: TradingAlgorithm, asset) -> str:
        finder = trading_algorithm.asset_finder
        cached = getattr(self, "_ticker_history", None)
        if cached is None or cached[0] is not finder:
            cached = (finder, ticker_history(finder))
            self._ticker_history = cached
        tickers = cached[1].get(int(asset.sid))
        if not tickers:
            return asset.symbol
        return ticker_on(tickers, _session_day(trading_algorithm.datetime))

    def _get_execution_result(self, data: bytes) -> bytes:
        req = engine_service_pb.GetResultRequest()
        merge_from_binary(req, data)
        assert self.result is not None, "no result"
        assert type(self.result) is pd.DataFrame, "result is not a dataframe"
        rsp = engine_service_pb.GetResultResponse()
        for row in self.result.index:
            period = self.result.loc[row]
            rsp.periods.append(
                execution_pb.Period(
                    date=pb_utils.from_pydate_to_proto_date(period["period_close"].to_pydatetime().replace()),
                    PNL=period["pnl"],
                    returns=period["returns"],
                    portfolio_value=period["portfolio_value"],
                    longs_count=int(period["longs_count"]),
                    shorts_count=int(period["shorts_count"]),
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
        upload = req.upload
        if upload is not None:
            storage = Storage.from_environment()
            self.result.to_pickle("/tmp/result.pkl")
            storage.upload_object(upload.bucket, upload.object, "/tmp/result.pkl")
        return rsp.to_binary()

    def _process_request(
        self,
        trading_algorithm: TradingAlgorithm | None,
        bar_data: BarData | None,
        socket: pynng.Socket,
    ):
        while True:
            with socket.new_context() as context_socket:
                req = common_pb.Request()
                merge_from_binary(req, context_socket.recv())
                self.log.debug(f"Received request {req.task}, {id(trading_algorithm)}")
                rsp = common_pb.Response(task=req.task)
                data: bytes | None = None
                try:
                    match req.task:
                        case "run":
                            if trading_algorithm is not None:
                                rsp.error = "Execution already running"
                                context_socket.send(rsp.to_binary())
                            else:
                                ta, data = self._run(req.data, socket)
                                rsp.data = data
                                context_socket.send(rsp.to_binary())
                                ta.run()
                                return
                        case "get_current_period":
                            if trading_algorithm is None:
                                rsp.data = engine_service_pb.GetCurrentPeriodResponse(is_running=False).to_binary()
                                context_socket.send(rsp.to_binary())
                            else:
                                rsp.data = self._get_current_period(req.data, trading_algorithm)
                                context_socket.send(rsp.to_binary())
                        case "place_orders_and_continue":
                            if trading_algorithm is None:
                                rsp.error = "Execution not running"
                                context_socket.send(rsp.to_binary())
                            else:
                                rsp.data = self._place_orders(req.data, trading_algorithm)
                                context_socket.send(rsp.to_binary())
                                return
                        case "stop":
                            context_socket.send(rsp.to_binary())
                            raise StopExcecution()
                        case "get_result":
                            rsp.data = self._get_execution_result(req.data)
                            context_socket.send(rsp.to_binary())
                except StopExcecution:
                    raise
                except PlainSentence as e:
                    self.log.warning("Refused request %s: %s", req.task, e)
                    rsp.error = PLAIN_SENTENCE_TAG + str(e)
                    context_socket.send(rsp.to_binary())
                except Exception as e:
                    self.log.exception("Error processing request %s", req.task)
                    rsp.error = str(e)
                    context_socket.send(rsp.to_binary())
