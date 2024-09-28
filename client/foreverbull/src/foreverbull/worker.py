import logging
import logging.handlers
import multiprocessing
import multiprocessing.synchronize
import os
import threading
from abc import ABC, abstractmethod
from functools import wraps
from multiprocessing import Process, Queue
from multiprocessing.synchronize import Event

import pynng
import sqlalchemy
from foreverbull import exceptions, models
from foreverbull.models import get_engine
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull.pb.foreverbull.service import (
    service_pb2,
    worker_pb2,
    worker_service_pb2,
)


class Worker(ABC):
    @abstractmethod
    def configure_execution(self, req: worker_pb2.ExecutionConfiguration) -> None:
        pass

    @abstractmethod
    def run_execution(self, stop_event: Event | threading.Event) -> None:
        pass


class WorkerInstance(Worker):
    def __init__(self, file_path: str):
        self.logger = logging.getLogger(__name__)
        self._file_path = file_path

        self._broker_socket: pynng.Socket | None = None
        self._namespace_socket: pynng.Socket | None = None
        self._database_engine: sqlalchemy.Engine | None = None

    def configure_execution(self, req: worker_pb2.ExecutionConfiguration) -> None:
        self.logger.info("configuring worker")
        try:
            self._algo = models.Algorithm.from_file_path(self._file_path)
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to load algorithm: {e}")

        _hostname = os.getenv("BROKER_HOSTNAME", "127.0.0.1")
        try:
            self._broker_socket = pynng.Rep0(
                dial=f"tcp://{_hostname}:{req.brokerPort}",
                block_on_dial=True,
                recv_timeout=500,
                send_timeout=500,
            )
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to broker: {e}")
        try:
            self._namespace_socket = pynng.Req0(
                dial=f"tcp://{_hostname}:{req.namespacePort}",
                block_on_dial=True,
                recv_timeout=500,
                send_timeout=500,
            )
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to namespace: {e}")

        try:
            engine = get_engine(req.databaseURL)
            with engine.connect() as connection:
                connection.execute(sqlalchemy.text("SELECT 1 from asset;"))
            self._database_engine = engine
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to database: {e}")

        for function in req.functions:
            for parameter in function.parameters:
                self._algo.configure(function.name, parameter.key, parameter.value)

        self.logger.info("worker configured correctly")

    @property
    def is_configured(self) -> bool:
        return (
            self._database_engine is not None
            and self._broker_socket is not None
            and self._namespace_socket is not None
        )

    def run_execution(self, stop_event: Event | threading.Event) -> None:
        if (
            not self._database_engine
            or not self._broker_socket
            or not self._namespace_socket
        ):
            raise exceptions.ConfigurationError("Worker not configured")

        while not stop_event.is_set():
            request = None
            self.logger.debug("Getting context socket")
            context_socket = self._broker_socket.new_context()
            try:
                request = worker_service_pb2.WorkerRequest()
                request.ParseFromString(context_socket.recv())
                response = worker_service_pb2.WorkerResponse(
                    task=request.task, error=None
                )
                self.logger.info("Processing symbols: %s", request.symbols)
                with self._database_engine.connect() as db:
                    orders = self._algo.process(
                        request.task,
                        db,
                        request.portfolio,
                        [symbol for symbol in request.symbols],
                    )
                self.logger.info("Sending orders to broker: %s", orders)
                for order in orders:
                    response.orders.append(
                        finance_pb2.Order(symbol=order.symbol, amount=order.amount)
                    )
                context_socket.send(response.SerializeToString())
                context_socket.close()
            except pynng.exceptions.Timeout:
                context_socket.close()
            except Exception as e:
                self.logger.exception(repr(e))
                if request:
                    response = worker_service_pb2.WorkerResponse()
                    response.error = repr(e)
                    context_socket.send(response.SerializeToString())
                if context_socket:
                    context_socket.close()
        self._broker_socket.close()
        self._namespace_socket.close()
        return None


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
        if self._logging_queue:
            handler = logging.handlers.QueueHandler(self._logging_queue)
            logging.basicConfig(level=logging.DEBUG, handlers=[handler])
        self.logger = logging.getLogger(__name__)
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
                self.logger.info(f"Received request: {request.task}")
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
            self.logger.info(f"Request processed: {request.task}")
        responder.close()
        return


class WorkerPool(Worker):
    def __init__(self, file_path: str, executors: int = 2):
        self._file_path = file_path
        self._executors = executors

        self._worker_surveyor_address = "ipc:///tmp/worker_pool.ipc"
        self._worker_surveyor_socket: pynng.Surveyor0
        self._workers: list[threading.Thread | Process] = []
        self.logger = logging.getLogger(__name__)
        self._log_queue = Queue()
        self._log_thread = threading.Thread(
            target=self.logger_thread, args=(self._log_queue,), daemon=True
        )
        self._stop_event: threading.Event | multiprocessing.synchronize.Event | None = (
            None
        )

    @staticmethod
    def logger_thread(queue: Queue):
        handler = logging.handlers.QueueHandler(queue)
        logging.basicConfig(level=logging.DEBUG, handlers=[handler])
        logger = logging.getLogger(__name__)
        while True:
            try:
                record = queue.get()
                if record is None:
                    break
                logger.handle(record)
            except Exception as e:
                logger.error(f"Error in logger thread: {e}")

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
        if os.getenv("THREADED_EXECUTION"):
            stop_event = threading.Event()
            for _ in range(self._executors):
                is_ready = threading.Event()
                daemon = WorkerDaemon(
                    self._worker_surveyor_address,
                    self._log_queue,
                    is_ready,
                    stop_event,
                    self._file_path,
                )
                t = threading.Thread(target=daemon.run, args=())
                t.start()
                if not is_ready.wait(5.0):
                    raise exceptions.ConfigurationError("Worker failed to start")
                self._workers.append(t)
            self._stop_event = stop_event
        else:
            stop_event = multiprocessing.Event()
            for _ in range(self._executors):
                is_ready = multiprocessing.Event()
                daemon = WorkerDaemon(
                    self._worker_surveyor_address,
                    self._log_queue,
                    is_ready,
                    stop_event,
                    self._file_path,
                )
                p = Process(target=daemon.run, args=())
                p.start()
                if not is_ready.wait(5.0):
                    raise exceptions.ConfigurationError("Worker failed to start")
                self._workers.append(p)
            self._stop_event = stop_event
        self._log_thread.start()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        if self._stop_event is None:
            return
        self._stop_event.set()
        self._log_queue.put_nowait(None)
        self._log_thread.join()
        for w in self._workers:
            w.join()
        self._worker_surveyor_socket.close()

    @staticmethod
    def _is_running(func):
        @wraps(func)
        def wrapper(w, *args, **kwargs):
            if w._stop_event is None or w._stop_event.is_set():
                raise RuntimeError("WorkerPool is not running")
            return func(w, *args, **kwargs)

        return wrapper

    @_is_running
    def configure_execution(self, req: worker_pb2.ExecutionConfiguration):
        data = common_pb2.Request(task="configure", data=req.SerializeToString())
        self._worker_surveyor_socket.send(data.SerializeToString())
        responders = 0
        while True:
            try:
                msg = self._worker_surveyor_socket.recv()
                response = common_pb2.Response()
                response.ParseFromString(msg)
                if response.HasField("error"):
                    raise exceptions.ConfigurationError(
                        f"Worker error: {response.error}"
                    )
                responders += 1
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                pass
        if responders != len(self._workers):
            raise RuntimeError("Not all workers responded to configure request")

    @_is_running
    def run_execution(self, stop_event: Event | threading.Event):
        data = common_pb2.Request(task="run")
        self._worker_surveyor_socket.send(data.SerializeToString())
        responders = 0
        while True:
            try:
                msg = self._worker_surveyor_socket.recv()
                response = common_pb2.Response()
                response.ParseFromString(msg)
                if response.HasField("error"):
                    raise exceptions.ConfigurationError(
                        f"Worker error: {response.error}"
                    )
                responders += 1
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                pass
        if responders != len(self._workers):
            raise Exception("Not all workers responded to run request")
