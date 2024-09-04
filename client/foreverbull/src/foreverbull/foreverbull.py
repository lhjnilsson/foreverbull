import logging
import os
import threading
import time
from contextlib import contextmanager
from multiprocessing import Event, Queue, synchronize

import pynng
from foreverbull import Algorithm, entity, worker
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, engine_pb2
from foreverbull.pb.service import service_pb2
from pandas import DataFrame

from .exceptions import ConfigurationError


class Session(threading.Thread):
    def __init__(
        self,
        algorithm: entity.service.Service.Algorithm,
        surveyor: pynng.Surveyor0,
        states: pynng.Sub0,
        workers: list[worker.Worker],
        stop_event: synchronize.Event,
    ):
        self._algorithm = algorithm
        self._surveyor = surveyor
        self._states = states
        self._workers = workers
        self._stop_event = stop_event
        self.logger = logging.getLogger(__name__)
        threading.Thread.__init__(self)

    def _configure_execution(self, data: bytes):
        self.logger.info("configuring workers")
        request = service_pb2.Request(task="configure_execution", data=data)
        self._surveyor.send(request.SerializeToString())
        responders = 0
        while True:
            try:
                rsp = service_pb2.Response()
                rsp.ParseFromString(self._surveyor.recv())
                if rsp.HasField("error"):
                    raise ConfigurationError(rsp.error)
                responders += 1
                self.logger.info("worker %s configured", responders)
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                raise ConfigurationError("Workers did not respond in time for configuration")
        self.logger.info("all workers configured")

    def _get_broker_session_socket(self):
        broker_hostname = os.getenv("BROKER_HOSTNAME", "127.0.0.1")
        broker_session_port = os.getenv("BROKER_SESSION_PORT")
        if broker_session_port is None:
            raise ConfigurationError("BROKER_SESSION_PORT not set")
        socket = pynng.Req0(dial=f"tcp://{broker_hostname}:{broker_session_port}", block_on_dial=True)
        socket.send_timeout = 5000
        socket.recv_timeout = 5000
        return socket

    def new_backtest_execution(self) -> entity.backtest.Execution:
        sock = self._get_broker_session_socket()
        algorithm = service_pb2.Algorithm(
            file_path=self._algorithm.file_path,
            namespaces=self._algorithm.namespaces,
        )
        for function in self._algorithm.functions:
            parameters = [
                service_pb2.Algorithm.FunctionParameter(
                    key=p.key,
                    defaultValue=p.default,
                    valueType=p.type,
                )
                for p in function.parameters
            ]
            algorithm.functions.append(
                service_pb2.Algorithm.Function(
                    name=function.name,
                    parameters=parameters,
                    runFirst=function.run_first,
                    runLast=function.run_last,
                )
            )

        exc_request = backtest_pb2.NewExecutionRequest(algorithm=algorithm)
        request = service_pb2.Request(task="new_execution", data=exc_request.SerializeToString())
        sock.send(request.SerializeToString())
        rsp = service_pb2.Response()
        rsp.ParseFromString(sock.recv())
        if rsp.HasField("error"):
            raise Exception(rsp.error)
        exc = backtest_pb2.NewExecutionResponse()
        exc.ParseFromString(rsp.data)
        return entity.backtest.Execution(
            id=exc.id,
            start=exc.start_date.ToDatetime(),
            end=exc.end_date.ToDatetime(),
            benchmark=None,
            symbols=[s for s in exc.symbols],
        )

    def _run_execution(self):
        req = service_pb2.Request(task="run_execution")
        self._surveyor.send(req.SerializeToString())
        responders = 0
        while True:
            try:
                self._surveyor.recv()
                responders += 1
                self.logger.info("worker %s executing", responders)
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                raise Exception("Workers did not respond in time for execution")
        self.logger.info("all workers executing")

    def run_backtest_execution(self, execution: entity.backtest.Execution):
        sock = self._get_broker_session_socket()
        if execution.id is None:
            raise Exception("Execution ID not set")
        config = backtest_pb2.ConfigureExecutionRequest(
            execution=execution.id,
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
        request = service_pb2.Request(task="configure_execution", data=config.SerializeToString())
        sock.send(request.SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(sock.recv())
        if response.HasField("error"):
            raise Exception(response.error)
        self._configure_execution(response.data)
        self._run_execution()
        run_request = backtest_pb2.RunExecutionRequest(execution=execution.id)
        request = service_pb2.Request(task="run_execution", data=run_request.SerializeToString())
        sock.send(request.SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(sock.recv())
        if response.HasField("error"):
            raise Exception(response.error)
        time.sleep(2)
        while True:
            request = service_pb2.Request(task="current_portfolio")
            sock.send(request.SerializeToString())
            response = service_pb2.Response()
            response.ParseFromString(sock.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            if not response.HasField("data"):
                break
            period = engine_pb2.GetPortfolioResponse()
            period.ParseFromString(response.data)
            self.logger.info("current period: %s", period.timestamp.ToDatetime())
        request = service_pb2.Request(task="stop")
        sock.send(request.SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(sock.recv())
        if response.HasField("error"):
            raise Exception(response.error)

    def run(self):
        local_port = os.environ.get("LOCAL_PORT", 5555)
        sock = pynng.Rep0(listen=f"tcp://0.0.0.0:{local_port}")
        sock.recv_timeout = 300
        while not self._stop_event.is_set():
            ctx = sock.new_context()
            try:
                try:
                    b = ctx.recv()
                except Exception:
                    continue
                try:
                    req = service_pb2.Request()
                    req.ParseFromString(b)
                except Exception as e:
                    self.logger.warning("Error deserializing request: %s", repr(e))
                    continue
                self.logger.info("received request: %s", req)
                response = service_pb2.Response(task=req.task)
                try:
                    match req.task:
                        case "info":
                            algorithm = service_pb2.Algorithm(
                                file_path=self._algorithm.file_path,
                                functions=[
                                    service_pb2.Algorithm.Function(
                                        name=function.name,
                                        parameters=[
                                            service_pb2.Algorithm.FunctionParameter(
                                                key=param.key,
                                                defaultValue=param.default,
                                                valueType=param.type,
                                            )
                                            for param in function.parameters
                                        ],
                                        parallelExecution=function.parallel_execution,
                                        runFirst=function.run_first,
                                        runLast=function.run_last,
                                    )
                                    for function in self._algorithm.functions
                                ],
                                namespaces=self._algorithm.namespaces,
                            )
                            service_info = service_pb2.ServiceInfoResponse(
                                serviceType="worker",
                                version="0.0.0",
                                algorithm=algorithm,
                            )
                            response.data = service_info.SerializeToString()
                            ctx.send(response.SerializeToString())
                        case "configure_execution":
                            self._configure_execution(req.data)
                            ctx.send(response.SerializeToString())
                        case "run_execution":
                            self._run_execution()
                            ctx.send(response.SerializeToString())
                        case "stop":
                            ctx.send(response.SerializeToString())
                            break
                        case _:
                            response.error = "Unknown task"
                            ctx.send(response.SerializeToString())
                except Exception as e:
                    response.error = repr(e)
                    ctx.send(response.SerializeToString())
            except pynng.exceptions.Timeout:
                pass
            except Exception as e:
                self.logger.error("Error in socket runner: %s", repr(e))
            finally:
                ctx.close()
        sock.close()


def logging_thread(q: Queue):
    while True:
        record = q.get()
        if record is None:
            break
        logger = logging.getLogger(record.name)
        logger.handle(record)


from abc import ABC, abstractmethod
from contextlib import contextmanager


class BacktestExecution(ABC):
    @abstractmethod
    def get_default(self) -> entity.backtest.Execution:
        pass

    @abstractmethod
    def run(self, execution: entity.backtest.Execution) -> DataFrame:
        pass


class Foreverbull(ABC):
    @abstractmethod
    def get_info(self, request: service_pb2.GetServiceInfoRequest) -> service_pb2.GetServiceInfoResponse:
        pass

    @abstractmethod
    def configure_execution(
        self, request: service_pb2.ConfigureExecutionRequest
    ) -> service_pb2.ConfigureExecutionResponse:
        pass

    @abstractmethod
    def run_execution(self, request: service_pb2.RunExecutionRequest) -> service_pb2.RunExecutionResponse:
        pass

    @abstractmethod
    def stop(self, request: service_pb2.StopRequest) -> service_pb2.StopResponse:
        pass

    @abstractmethod
    def new_backtest_execution(self) -> BacktestExecution:
        pass


class _Foreverbull:
    def __init__(self, file_path: str | None = None, executors=2):
        self._session = None
        self._file_path = file_path
        if self._file_path:
            try:
                Algorithm.from_file_path(self._file_path)
            except Exception as e:
                raise ImportError(f"Could not import file {file_path}: {repr(e)}")
        self._executors = executors

        self._worker_surveyor_address = "ipc:///tmp/worker_pool.ipc"
        self._worker_surveyor_socket: pynng.Surveyor0 | None = None
        self._worker_states_address = "ipc:///tmp/worker_states.ipc"
        self._worker_states_socket: pynng.Sub0 | None = None
        self._stop_event: synchronize.Event | None = None
        self._workers = []
        self.logger = logging.getLogger(__name__)

    def __enter__(self) -> Session:
        if self._file_path is None:
            raise Exception("No algo file provided")
        algo = Algorithm.from_file_path(self._file_path)
        self._worker_surveyor_socket = pynng.Surveyor0(listen=self._worker_surveyor_address)
        self._worker_surveyor_socket.send_timeout = 30000
        self._worker_surveyor_socket.recv_timeout = 30000
        self._worker_states_socket = pynng.Sub0(listen=self._worker_states_address)
        self._worker_states_socket.subscribe(b"")
        self._worker_states_socket.recv_timeout = 30000
        self._log_queue = Queue()
        self._log_thread = threading.Thread(target=logging_thread, args=(self._log_queue,))
        self._log_thread.start()
        self._stop_event = Event()
        self.logger.info("starting workers")
        for i in range(self._executors):
            self.logger.info("starting worker %s", i)
            if os.getenv("THREADED_EXECUTION"):
                w = worker.WorkerThread(
                    self._worker_surveyor_address,
                    self._worker_states_address,
                    self._log_queue,
                    self._stop_event,
                    algo.get_entity().file_path,
                )
            else:
                w = worker.WorkerProcess(
                    self._worker_surveyor_address,
                    self._worker_states_address,
                    self._log_queue,
                    self._stop_event,
                    algo.get_entity().file_path,
                )
            w.start()
            self._workers.append(w)
        responders = 0
        while True:
            try:
                self._worker_states_socket.recv()
                self.logger.info("worker %s started", responders)
                responders += 1
                if responders == self._executors:
                    break
            except pynng.exceptions.Timeout:
                raise Exception("Workers did not respond in time")
        self.logger.info("workers started")
        s = Session(
            algo.get_entity(),
            self._worker_surveyor_socket,
            self._worker_states_socket,
            self._workers,
            self._stop_event,
        )
        s.start()
        self._session = s
        return s

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self._stop_event and not self._stop_event.is_set():
            self._stop_event.set()
        self._log_queue.put_nowait(None)
        [worker.join() for worker in self._workers]
        self._log_thread.join()
        self.logger.info("workers stopped")
        if self._worker_surveyor_socket:
            self._worker_surveyor_socket.close()
        if self._worker_states_socket:
            self._worker_states_socket.close()
        self._stop_event = None
        if self._session:
            self._session.join()
            self._session = None
