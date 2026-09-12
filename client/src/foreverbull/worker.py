import logging
import logging.handlers
import multiprocessing
import multiprocessing.synchronize
import os
import threading
import time

from abc import ABC
from abc import abstractmethod
from functools import wraps
from multiprocessing import Process
from multiprocessing import Queue
from multiprocessing.synchronize import Event

import pynng
import sqlalchemy

from foreverbull import exceptions
from foreverbull import models
from foreverbull.broker.utils import get_engine
from foreverbull.broker.utils import nng_hostname
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb2 as worker_service_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2


class Worker(ABC):
    @abstractmethod
    def configure_execution(self, req: worker_pb2.ExecutionConfiguration) -> None:
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
        self._database_engine: sqlalchemy.Engine | None = None

    def _dial(self, socket_cls, port: int):
        hostname = nng_hostname()
        return socket_cls(
            dial=f"tcp://{hostname}:{port}",
            block_on_dial=True,
            recv_timeout=500,
            send_timeout=500,
        )

    def configure_execution(self, req: worker_pb2.ExecutionConfiguration) -> None:
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

        try:
            engine = get_engine(req.databaseURL)
            with engine.connect() as connection:
                connection.execute(sqlalchemy.text("SELECT 1 from asset;"))
            self._database_engine = engine
            self.logger.debug(f"connected to database: {req.databaseURL}")
        except Exception as e:
            raise exceptions.ConfigurationError("Unable to connect to database") from e

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
                self._database_engine is not None,
                self._broker_socket is not None,
                self._namespace_socket is not None,
            ]
        )

    def run_execution(self, stop_event: Event | threading.Event) -> None:
        self.logger.info("running execution")
        if not self._algo or not self._database_engine or not self._broker_socket or not self._namespace_socket:
            raise exceptions.ConfigurationError("Worker not configured")
        while not stop_event.is_set():
            request = None
            self.logger.debug("Getting context socket")
            context_socket = self._broker_socket.new_context()
            try:
                request = worker_service_pb2.WorkerRequest()
                request.ParseFromString(context_socket.recv())
                response = worker_service_pb2.WorkerResponse(task=request.task, error=None)
                self.logger.debug(f"Processing {request.portfolio.timestamp} symbols: {request.symbols}")
                with self._database_engine.connect() as db:
                    orders = self._algo.process(
                        request.task,
                        db,
                        request.portfolio,
                        [symbol for symbol in request.symbols],
                    )
                self.logger.debug("Sending orders to broker: %s", orders)
                for order in orders:
                    response.orders.append(marketdata_pb2.Order(symbol=order.symbol, amount=order.amount))
                context_socket.send(response.SerializeToString())
            except pynng.exceptions.Timeout:
                pass
            except Exception as e:
                self.logger.exception(repr(e))
                if request:
                    response = worker_service_pb2.WorkerResponse()
                    response.error = repr(e)
                    context_socket.send(response.SerializeToString())
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
    ):
        self._survey_address = survey_address
        self._logging_queue = logging_queue
        self._running_event = running_event
        self._stop_event = stop_event
        super().__init__(file_path)

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
        except Exception as e:
            self.logger.error(f"Unable to connect to surveyor: {e}")
            return

        self._running_event.set()
        self.logger.info("starting worker")
        while not self._stop_event.is_set():
            request = common_pb2.Request()
            try:
                request.ParseFromString(responder.recv())
                self.logger.debug(f"Received request: {request.task}")
                if request.task == "configure":
                    req = worker_pb2.ExecutionConfiguration()
                    req.ParseFromString(request.data)
                    self.configure_execution(req)
                    response = common_pb2.Response(task=request.task, error=None)
                    responder.send(response.SerializeToString())
                elif request.task == "run":
                    if not self.is_configured:
                        raise exceptions.ConfigurationError("Worker not configured")
                    response = common_pb2.Response(task=request.task, error=None)
                    responder.send(response.SerializeToString())
                    self.run_execution(self._stop_event)
            except pynng.exceptions.Timeout:
                continue
            except Exception as e:
                self.logger.error("Error processing request")
                self.logger.exception(repr(e))
                response = common_pb2.Response(task=request.task, error=repr(e))
                responder.send(response.SerializeToString())
            self.logger.debug(f"Request processed: {request.task}")
        responder.close()
        return


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
        threaded = bool(os.getenv("THREADED_EXECUTION"))
        if threaded:
            stop_event: threading.Event | multiprocessing.synchronize.Event = threading.Event()
            ready_event = threading.Event
        else:
            stop_event = multiprocessing.Event()
            ready_event = multiprocessing.Event

        for _ in range(self._executors):
            is_ready = ready_event()
            daemon = WorkerDaemon(
                self._worker_surveyor_address,
                self._log_queue,
                is_ready,
                stop_event,
                self._file_path,
            )
            worker = threading.Thread(target=daemon.run) if threaded else Process(target=daemon.run)
            worker.start()
            if not is_ready.wait(5.0):
                raise exceptions.ConfigurationError("Worker failed to start")
            self._workers.append(worker)
        self._stop_event = stop_event

    def __enter__(
        self,
    ):
        try:
            models.Algorithm.from_file_path(self._file_path)
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to load algorithm: {e}")
        self._worker_surveyor_socket = pynng.Surveyor0(
            listen=self._worker_surveyor_address, send_timeout=30000, recv_timeout=30000
        )
        self._start_daemons()
        self._log_listener.start()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        if self._stop_event is None:
            return
        self._stop_event.set()
        while not self._log_queue.empty():
            time.sleep(0.1)

        self._log_queue.put_nowait(None)
        self._log_listener.stop()
        self._log_queue.close()
        self._log_queue.join_thread()
        for w in self._workers:
            w.join(timeout=5)
            if w.is_alive():
                raise RuntimeError("Worker did not exit")
        self._worker_surveyor_socket.close()

    @staticmethod
    def _is_running(func):
        @wraps(func)
        def wrapper(w, *args, **kwargs):
            if w._stop_event is None or w._stop_event.is_set():
                raise RuntimeError("WorkerPool is not running")
            return func(w, *args, **kwargs)

        return wrapper

    def _broadcast(self, request: common_pb2.Request, incomplete: Exception) -> None:
        self._worker_surveyor_socket.send(request.SerializeToString())
        responders = 0
        while True:
            try:
                msg = self._worker_surveyor_socket.recv()
                response = common_pb2.Response()
                response.ParseFromString(msg)
                if response.HasField("error"):
                    raise exceptions.ConfigurationError(f"Worker error: {response.error}")
                responders += 1
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                pass
        if responders != len(self._workers):
            raise incomplete

    @_is_running
    def configure_execution(self, req: worker_pb2.ExecutionConfiguration):
        self._broadcast(
            common_pb2.Request(task="configure", data=req.SerializeToString()),
            RuntimeError("Not all workers responded to configure request"),
        )

    @_is_running
    def run_execution(self, stop_event: Event | threading.Event):
        self._broadcast(
            common_pb2.Request(task="run"),
            Exception("Not all workers responded to run request"),
        )
