import logging
import logging.handlers
import multiprocessing
import multiprocessing.synchronize
import os
import queue
import threading
import time

from abc import ABC
from abc import abstractmethod
from collections.abc import Sequence
from functools import wraps
from multiprocessing import Process
from multiprocessing import Queue
from multiprocessing.synchronize import Event

import pynng

from protobuf import merge_from_binary

from foreverbull import exceptions
from foreverbull import models
from foreverbull.broker.utils import describe_exception
from foreverbull.broker.utils import nng_hostname
from foreverbull.models.finance import worker_request_error
from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb as worker_service_pb
from foreverbull.pb.foreverbull.service.v1 import worker_pb


# How long the pool waits for every worker to answer a configure or run request.
SURVEY_TIMEOUT_MS = 30000
# How long the pool waits for each worker to connect before giving up on it.
WORKER_START_TIMEOUT_SECONDS = 5.0
# What each request asks the workers to do, as the pool's errors name it.
REQUEST_ACTIONS = {"configure": "finish setting up", "run": "start the backtest"}
# A startup error a worker reports to the pool: (kind, "Type: message"). kind is "connection" or "other".
StartupError = tuple[str, str]


class Worker(ABC):
    @abstractmethod
    def configure_execution(self, req: worker_pb.ExecutionConfiguration) -> None:
        pass

    @abstractmethod
    def run_execution(self, stop_event: Event | threading.Event) -> None:
        pass


class WorkerInstance(Worker):
    def __init__(self, file_path: str):
        self.logger = logging.getLogger().getChild(__name__)
        self._file_path = file_path
        self._algo: models.Algorithm | None = None

        self._broker_socket: pynng.Socket | None = None
        self._namespace_socket: pynng.Socket | None = None

    def _dial(self, socket_cls, port: int):
        hostname = nng_hostname()
        return socket_cls(
            dial=f"tcp://{hostname}:{port}",
            block_on_dial=True,
            recv_timeout=500,
            send_timeout=500,
        )

    def configure_execution(self, req: worker_pb.ExecutionConfiguration) -> None:
        self.logger.debug("configuring worker")
        try:
            self._algo = models.Algorithm.from_file_path(self._file_path)
            self.logger.debug(f"loaded algorithmic file: {self._file_path}")
        except Exception as e:
            raise exceptions.ConfigurationError("Unable to load algorithm") from e

        hostname = nng_hostname()
        try:
            self._broker_socket = self._dial(pynng.Rep0, req.brokerPort)
            self.logger.debug(f"connected to broker socket: tcp://{hostname}:{req.brokerPort}")
        except Exception as e:
            raise exceptions.ConfigurationError("Unable to connect to broker") from e
        try:
            self._namespace_socket = self._dial(pynng.Req0, req.namespacePort)
            self.logger.debug(f"connected to namespace socket: tcp://{hostname}:{req.namespacePort}")
        except Exception as e:
            raise exceptions.ConfigurationError("Unable to connect to namespace") from e

        # databaseURL stays on the wire for old brokers. The worker does not
        # open it, and a URL left in the process environment is removed.
        # Bars arrive on each WorkerRequest.
        os.environ.pop("DATABASE_URL", None)
        for function in req.functions:
            for parameter in function.parameters:
                self._algo.configure(function.name, parameter.key, parameter.value)
                self.logger.debug(
                    f"configured function {function.name} with parameter {parameter.key}={parameter.value}"
                )
        self.logger.info("worker configured correctly")

    @property
    def is_configured(self) -> bool:
        return all(
            [
                self._algo is not None,
                self._broker_socket is not None,
                self._namespace_socket is not None,
            ]
        )

    def run_execution(self, stop_event: Event | threading.Event) -> None:
        self.logger.info("running execution")
        if not self._algo or not self._broker_socket or not self._namespace_socket:
            raise exceptions.ConfigurationError("Worker not configured")
        while not stop_event.is_set():
            request = None
            self.logger.debug("Getting context socket")
            context_socket = self._broker_socket.new_context()
            try:
                request = worker_service_pb.WorkerRequest()
                merge_from_binary(request, context_socket.recv())
                response = worker_service_pb.WorkerResponse(task=request.task, error=None)
                portfolio = request.portfolio
                if portfolio is None:
                    raise exceptions.ConfigurationError("worker request has no portfolio")
                self.logger.debug(f"Processing {portfolio.timestamp} symbols: {request.symbols}")
                orders = self._algo.process(
                    request.task,
                    portfolio,
                    [symbol for symbol in request.symbols],
                    list(request.bars),
                )
                self.logger.debug("Sending orders to broker: %s", orders)
                for order in orders:
                    response.orders.append(marketdata_pb.Order(symbol=order.symbol, amount=order.amount))
                context_socket.send(response.to_binary())
            except pynng.exceptions.Timeout:
                pass
            except Exception as e:
                self.logger.exception("Worker request failed")
                if request:
                    response = worker_service_pb.WorkerResponse()
                    response.error = worker_request_error(e)
                    context_socket.send(response.to_binary())
            finally:
                context_socket.close()
        self._broker_socket.close()
        self._namespace_socket.close()


class WorkerDaemon(WorkerInstance):
    def __init__(
        self,
        survey_address: str,
        logging_queue: Queue,
        running_event: Event | threading.Event,
        stop_event: Event | threading.Event,
        file_path: str,
        startup_errors: "Queue | queue.Queue[StartupError] | None" = None,
    ):
        self._survey_address = survey_address
        self._logging_queue = logging_queue
        self._running_event = running_event
        self._stop_event = stop_event
        # The pool reads why a worker couldn't start from here; a raise in a child process never reaches it.
        self._startup_errors = startup_errors
        super().__init__(file_path)

    def _report_startup_error(self, kind: str, exc: BaseException) -> None:
        if self._startup_errors is not None:
            self._startup_errors.put((kind, describe_exception(exc)))

    def run(self):
        self.logger = logging.getLogger().getChild(__name__)
        level = os.environ.get("LOGGING_LEVEL", "INFO")
        logging.basicConfig(
            handlers=[
                logging.handlers.QueueHandler(self._logging_queue) if self._logging_queue else logging.StreamHandler()
            ],
            level=level,
        )
        try:
            responder = pynng.Respondent0(
                dial=self._survey_address,
                block_on_dial=True,
                send_timeout=500,
                recv_timeout=500,
            )
        except pynng.exceptions.NNGException as e:
            message = (
                f"A worker couldn't connect to foreverbull at {self._survey_address}: {e}. "
                "Run `fbenv status` to check that it's running."
            )
            self.logger.error(message)
            self._report_startup_error("connection", e)
            raise exceptions.ConnectionError(message, cause=describe_exception(e)) from e
        except Exception as e:
            self._report_startup_error("other", e)
            raise

        self._running_event.set()
        self.logger.info("starting worker")
        while not self._stop_event.is_set():
            request = common_pb.Request()
            try:
                merge_from_binary(request, responder.recv())
                self.logger.debug(f"Received request: {request.task}")
                if request.task == "configure":
                    req = worker_pb.ExecutionConfiguration()
                    merge_from_binary(req, request.data)
                    self.configure_execution(req)
                    response = common_pb.Response(task=request.task, error=None)
                    responder.send(response.to_binary())
                elif request.task == "run":
                    if not self.is_configured:
                        raise exceptions.ConfigurationError("Worker not configured")
                    response = common_pb.Response(task=request.task, error=None)
                    responder.send(response.to_binary())
                    self.run_execution(self._stop_event)
            except pynng.exceptions.Timeout:
                continue
            except Exception as e:
                self.logger.exception("Error processing request")
                response = common_pb.Response(task=request.task, error=describe_exception(e))
                responder.send(response.to_binary())
            self.logger.debug(f"Request processed: {request.task}")
        responder.close()


def _run_worker_process(daemon: WorkerDaemon) -> None:
    # The algorithm process is not a broker client. A bearer inherited from
    # fbull would let GetExecution run as the operator.
    os.environ.pop("FOREVERBULL_TOKEN", None)
    daemon.run()


class WorkerPool(Worker):
    def __init__(self, file_path: str, executors: int = multiprocessing.cpu_count()):
        self._file_path = file_path
        self._executors = executors

        self._worker_surveyor_address = "ipc:///tmp/worker_pool.ipc"
        self._worker_surveyor_socket: pynng.Surveyor0
        self._workers: list[threading.Thread | Process] = []
        self.logger = logging.getLogger().getChild(__name__)
        self._log_queue = Queue()
        self._log_listener = logging.handlers.QueueListener(self._log_queue, *logging.getLogger().handlers)
        self._stop_event: threading.Event | multiprocessing.synchronize.Event | None = None

    def _start_daemons(self) -> None:
        """Start the workers one by one. Raise ConfigurationError naming why a worker couldn't start.

        The caller stops the workers already started when this raises.
        """
        threaded = bool(os.getenv("THREADED_EXECUTION"))
        startup_errors: Queue | queue.Queue[StartupError]
        if threaded:
            stop_event: threading.Event | multiprocessing.synchronize.Event = threading.Event()
            ready_event = threading.Event
            startup_errors = queue.Queue()
        else:
            stop_event = multiprocessing.Event()
            ready_event = multiprocessing.Event
            startup_errors = Queue()
        self._stop_event = stop_event

        for _ in range(self._executors):
            is_ready = ready_event()
            daemon = WorkerDaemon(
                self._worker_surveyor_address,
                self._log_queue,
                is_ready,
                stop_event,
                self._file_path,
                startup_errors,
            )
            worker = (
                threading.Thread(target=daemon.run) if threaded else Process(target=_run_worker_process, args=(daemon,))
            )
            worker.start()
            self._workers.append(worker)
            self._wait_until_started(worker, is_ready, startup_errors)

    @staticmethod
    def _wait_until_started(
        worker: threading.Thread | Process,
        is_ready: threading.Event | multiprocessing.synchronize.Event,
        startup_errors: "Queue | queue.Queue[StartupError]",
    ) -> None:
        deadline = time.monotonic() + WORKER_START_TIMEOUT_SECONDS
        while not is_ready.wait(0.05):
            if not worker.is_alive() or time.monotonic() >= deadline:
                break
        if is_ready.is_set():
            return
        try:
            # A process that just exited may still be flushing its report into the queue.
            kind, cause = startup_errors.get(timeout=1.0)
        except queue.Empty:
            if worker.is_alive():
                kind, cause = "other", f"it didn't connect within {WORKER_START_TIMEOUT_SECONDS:g}s"
            else:
                exitcode = getattr(worker, "exitcode", None)
                kind, cause = "other", "it exited" if exitcode is None else f"it exited with code {exitcode}"
        if kind == "connection":
            hint = "Run `fbenv status` to check that foreverbull is running."
        else:
            hint = "Check the worker output above for an error in your algo."
        raise exceptions.ConfigurationError(f"A worker couldn't start: {cause.rstrip('.')}. {hint}")

    def __enter__(
        self,
    ):
        try:
            models.Algorithm.from_file_path(self._file_path)
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to load algorithm: {e}") from e
        self._worker_surveyor_socket = pynng.Surveyor0(
            listen=self._worker_surveyor_address, send_timeout=SURVEY_TIMEOUT_MS, recv_timeout=SURVEY_TIMEOUT_MS
        )
        # A survey closes after survey_time (nng's default is 1s), and any later recv raises BadState.
        # Keep the survey open as long as recv waits, so _broadcast's bound is this one timeout.
        self._worker_surveyor_socket.survey_time = SURVEY_TIMEOUT_MS
        # Listen before any worker starts, so a worker's startup log reaches the pool's handlers.
        self._log_listener.start()
        try:
            self._start_daemons()
        except BaseException:
            # __exit__ doesn't run when __enter__ raises, so stop what already started here.
            self._shutdown(strict=False)
            raise
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        if self._stop_event is None:
            return
        self._shutdown(strict=True)

    def _shutdown(self, *, strict: bool) -> None:
        """Stop the workers, the log listener and the surveyor socket.

        strict raises RuntimeError when a worker doesn't exit; otherwise a stuck process is terminated.
        """
        if self._stop_event is not None:
            self._stop_event.set()
        deadline = time.monotonic() + 5
        while not self._log_queue.empty() and time.monotonic() < deadline:
            time.sleep(0.1)

        self._log_queue.put_nowait(None)
        self._log_listener.stop()
        self._log_queue.close()
        self._log_queue.join_thread()
        stuck = False
        for w in self._workers:
            w.join(timeout=5)
            if w.is_alive():
                stuck = True
                if isinstance(w, Process):
                    w.terminate()
                    w.join(timeout=5)
        self._worker_surveyor_socket.close()
        if stuck and strict:
            raise RuntimeError("Worker did not exit")

    @staticmethod
    def _is_running(func):
        @wraps(func)
        def wrapper(w, *args, **kwargs):
            if w._stop_event is None or w._stop_event.is_set():
                raise RuntimeError("WorkerPool is not running")
            return func(w, *args, **kwargs)

        return wrapper

    def _broadcast(self, request: common_pb.Request) -> None:
        """Send request to every worker and wait until all of them answered or the survey timed out.

        Every answer is collected before anything is raised, so one message names every worker that failed.
        Raise ConfigurationError when a worker reported an error, and RuntimeError when workers only timed out.
        """
        self._worker_surveyor_socket.send(request.to_binary())
        total = len(self._workers)
        answered = 0
        errors: list[str] = []
        while answered < total:
            try:
                msg = self._worker_surveyor_socket.recv()
            except (pynng.exceptions.Timeout, pynng.exceptions.BadState):
                break
            response = common_pb.Response()
            merge_from_binary(response, msg)
            answered += 1
            if response.has_field("error"):
                errors.append(str(response.error))
        missing = total - answered
        if not errors and not missing:
            return
        message = _broadcast_failure(request.task, total, errors, missing)
        if errors:
            raise exceptions.ConfigurationError(message)
        raise RuntimeError(message)

    @_is_running
    def configure_execution(self, req: worker_pb.ExecutionConfiguration):
        self._broadcast(common_pb.Request(task="configure", data=req.to_binary()))

    @_is_running
    def run_execution(self, stop_event: Event | threading.Event):
        self._broadcast(common_pb.Request(task="run"))


def _plural_workers(count: int) -> str:
    return "worker" if count == 1 else "workers"


def _broadcast_failure(task: str, total: int, errors: Sequence[str], missing: int) -> str:
    """The user-facing message for workers that reported an error or didn't answer a request."""
    action = REQUEST_ACTIONS.get(task, f"answer the {task} request")
    timeout = f"{SURVEY_TIMEOUT_MS / 1000:g}s"
    hint = "Check the worker output above for an error in your algo."
    of_total = f"of {total} {_plural_workers(total)}"
    if not errors:
        return f"{missing} {of_total} didn't {action} within {timeout}. {hint}"
    if len(errors) == 1 and not missing:
        return f"1 {of_total} couldn't {action}: {errors[0].rstrip('.')}. {hint}"
    lines = [f"{len(errors) + missing} {of_total} couldn't {action}:"]
    lines.extend(f"  A worker reported: {error.rstrip('.')}." for error in errors)
    if missing:
        lines.append(f"  {missing} {_plural_workers(missing)} didn't answer within {timeout}.")
    lines.append(hint)
    return "\n".join(lines)
