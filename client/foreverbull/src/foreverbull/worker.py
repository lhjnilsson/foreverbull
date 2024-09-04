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
from foreverbull import Algorithm, exceptions
from foreverbull.data import get_engine
from foreverbull.pb import common_pb2
from foreverbull.pb.finance import finance_pb2
from foreverbull.pb.service import service_pb2
from sqlalchemy import text


class Worker(ABC):
    @abstractmethod
    def configure_execution(self, req: service_pb2.ConfigureExecutionRequest) -> service_pb2.ConfigureExecutionResponse:
        pass

    @abstractmethod
    def run_execution(self, req: service_pb2.RunExecutionRequest) -> service_pb2.RunExecutionResponse:
        pass


class WorkerPool(Worker):
    def __init__(self, file_path: str, executors: int = 2):
        self._file_path = file_path
        self._executors = executors

        self._worker_surveyor_address = "ipc:///tmp/worker_pool.ipc"
        self._worker_surveyor_socket: pynng.Surveyor0
        self._workers: list[WorkerThread | WorkerProcess] = []
        self.logger = logging.getLogger(__name__)
        self._log_queue = Queue()
        self._stop_event: threading.Event | multiprocessing.synchronize.Event | None = None

    def __enter__(
        self,
    ):
        algo = Algorithm.from_file_path(self._file_path)
        self._worker_surveyor_socket = pynng.Surveyor0(
            listen=self._worker_surveyor_address, send_timeout=30000, recv_timeout=30000
        )
        if os.getenv("THREADED_EXECUTION"):
            stop_event = threading.Event()
            for _ in range(self._executors):
                w = WorkerThread(
                    survey_address=self._worker_surveyor_address,
                    logging_queue=self._log_queue,
                    stop_event=stop_event,
                    file_path=self._file_path,
                )
                w.is_ready.wait(1.0)
                w.start()
                self._workers.append(w)
            self._stop_event = stop_event
        else:
            stop_event = multiprocessing.Event()
            for _ in range(self._executors):
                w = WorkerProcess(
                    survey_address=self._worker_surveyor_address,
                    logging_queue=self._log_queue,
                    stop_event=stop_event,
                    file_path=self._file_path,
                )
                w.is_ready.wait(1.0)
                w.start()
                self._workers.append(w)
            self._stop_event = stop_event
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        if self._stop_event is None:
            return
        self._stop_event.set()
        self._log_queue.put_nowait(None)
        for w in self._workers:
            w.join()
        self._worker_surveyor_socket.close()

    @staticmethod
    def _is_running(func):
        @wraps(func)
        def wrapper(w: WorkerPool, *args, **kwargs):
            if w._stop_event is None or w._stop_event.is_set():
                raise Exception("WorkerPool is not running")
            return func(w, *args, **kwargs)

        return wrapper

    @_is_running
    def configure_execution(self, req: service_pb2.ConfigureExecutionRequest) -> service_pb2.ConfigureExecutionResponse:
        data = common_pb2.Request(task="configure", data=req.SerializeToString())
        self._worker_surveyor_socket.send(data.SerializeToString())
        responders = 0
        while True:
            try:
                msg = self._worker_surveyor_socket.recv()
                response = common_pb2.Response()
                response.ParseFromString(msg)
                responders += 1
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                pass
        if responders != len(self._workers):
            raise Exception("Not all workers responded to configure request")
        return service_pb2.ConfigureExecutionResponse()

    @_is_running
    def run_execution(self, req: service_pb2.RunExecutionRequest) -> service_pb2.RunExecutionResponse:
        data = common_pb2.Request(task="run", data=req.SerializeToString())
        self._worker_surveyor_socket.send(data.SerializeToString())
        responders = 0
        while True:
            try:
                msg = self._worker_surveyor_socket.recv()
                response = common_pb2.Response()
                response.ParseFromString(msg)
                responders += 1
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                pass
        if responders != len(self._workers):
            raise Exception("Not all workers responded to run request")
        return service_pb2.RunExecutionResponse()


class WorkerInstance(Worker):
    def __init__(self, survey_address: str, logging_queue: Queue, stop_event: Event | threading.Event, file_path: str):
        self._survey_address = survey_address
        self._logging_queue = logging_queue
        self._stop_event = stop_event
        self._database = None
        self._file_path = file_path
        self._parallel = False
        self.is_ready: threading.Event | multiprocessing.synchronize.Event
        super(WorkerInstance, self).__init__()

    def configure_execution(self, req: service_pb2.ConfigureExecutionRequest) -> service_pb2.ConfigureExecutionResponse:
        self.logger.info("configuring worker")
        self._algo = Algorithm.from_file_path(self._file_path)
        try:
            self.socket = pynng.Rep0(
                dial=f"tcp://{os.getenv('BROKER_HOSTNAME', '127.0.0.1')}:{req.brokerPort}", block_on_dial=True
            )
            self.socket.recv_timeout = 5000
            self.socket.send_timeout = 5000
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to broker: {e}")

        for function in req.functions:
            for parameter in function.parameters:
                self._algo.configure(function.name, parameter.key, parameter.value)

        try:
            engine = get_engine(req.databaseURL)
            with engine.connect() as connection:
                connection.execute(text("SELECT 1 from asset;"))
            self._database_engine = engine
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to database: {e}")

        os.environ["NAMESPACE_PORT"] = str(req.namespacePort)
        self.logger.info("worker configured correctly")
        return service_pb2.ConfigureExecutionResponse()

    def run(self):
        if self._logging_queue:
            handler = logging.handlers.QueueHandler(self._logging_queue)
            logging.basicConfig(level=logging.DEBUG, handlers=[handler])
        self.logger = logging.getLogger(__name__)
        try:
            responder = pynng.Respondent0(
                dial=self._survey_address, block_on_dial=True, send_timeout=500, recv_timeout=500
            )
        except Exception as e:
            self.logger.error(f"Unable to connect to surveyor: {e}")
            return

        self.is_ready.set()
        self.logger.info("starting worker")
        while not self._stop_event.is_set():
            request = common_pb2.Request()
            try:
                request.ParseFromString(responder.recv())
                self.logger.info(f"Received request: {request.task}")
                if request.task == "configure":
                    req = service_pb2.ConfigureExecutionRequest()
                    req.ParseFromString(request.data)
                    self.configure_execution(req)
                    response = common_pb2.Response(task=request.task, error=None)
                    responder.send(response.SerializeToString())
                elif request.task == "run":
                    response = common_pb2.Response(task=request.task, error=None)
                    responder.send(response.SerializeToString())
                    self.run_execution(service_pb2.RunExecutionRequest())
            except pynng.exceptions.Timeout:
                continue
            except Exception as e:
                self.logger.error("Error processing request")
                self.logger.exception(repr(e))
                response = common_pb2.Response(task=request.task, error=repr(e))
                responder.send(response.SerializeToString())
            self.logger.info(f"Request processed: {request.task}")
        responder.close()

    def run_execution(self, req: service_pb2.RunExecutionRequest) -> service_pb2.RunExecutionResponse:
        while True:
            request = None
            self.logger.debug("Getting context socket")
            context_socket = self.socket.new_context()
            try:
                request = service_pb2.WorkerRequest()
                request.ParseFromString(context_socket.recv())
                response = service_pb2.WorkerResponse(task=request.task, error=None)
                self.logger.info("Processing symbols: %s", request.symbols)
                with self._database_engine.connect() as db:
                    orders = self._algo.process(
                        request.task,
                        db,
                        request.portfolio,
                        request.timestamp.ToDatetime(),
                        [symbol for symbol in request.symbols],
                    )
                self.logger.info("Sending orders to broker: %s", orders)
                for order in orders:
                    response.orders.append(finance_pb2.Order(**order.model_dump()))
                context_socket.send(response.SerializeToString())
                context_socket.close()
            except pynng.exceptions.Timeout:
                context_socket.close()
            except Exception as e:
                self.logger.exception(repr(e))
                if request:
                    response = service_pb2.WorkerResponse()
                    response.error = repr(e)
                    context_socket.send(response.SerializeToString())
                if context_socket:
                    context_socket.close()
            if self._stop_event.is_set():
                break
        self.socket.close()
        return service_pb2.RunExecutionResponse()


class WorkerThread(WorkerInstance, threading.Thread):  # type: ignore
    def __init__(self, survey_address: str, logging_queue: Queue, stop_event: threading.Event, file_path: str):
        self.is_ready = threading.Event()
        WorkerInstance.__init__(self, survey_address, logging_queue, stop_event, file_path)

    def configure_execution(self, req: service_pb2.ConfigureExecutionRequest) -> service_pb2.ConfigureExecutionResponse:
        return super().configure_execution(req)

    def run_execution(self, req: service_pb2.RunExecutionRequest) -> service_pb2.RunExecutionResponse:
        return super().run_execution(req)


class WorkerProcess(WorkerInstance, Process):  # type: ignore
    def __init__(self, survey_address: str, logging_queue: Queue, stop_event: Event, file_path: str):
        self.is_ready = multiprocessing.Event()
        WorkerInstance.__init__(self, survey_address, logging_queue, stop_event, file_path)

    def configure_execution(self, req: service_pb2.ConfigureExecutionRequest) -> service_pb2.ConfigureExecutionResponse:
        return super().configure_execution(req)

    def run_execution(self, req: service_pb2.RunExecutionRequest) -> service_pb2.RunExecutionResponse:
        return super().run_execution(req)
