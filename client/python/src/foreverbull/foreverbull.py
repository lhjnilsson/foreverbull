import logging
import os
import threading
from multiprocessing import Event, Queue, synchronize

import pynng

from foreverbull import Algorithm, entity, worker
from foreverbull.pb_gen import service_pb2

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

    def _configure_execution(self, instance: entity.service.Instance):
        self.logger.info("configuring workers")
        request = service_pb2.Message(task="configure_execution")
        request.data.update(instance.model_dump())
        self._surveyor.send(request.SerializeToString())
        responders = 0
        while True:
            try:
                rsp = service_pb2.Message()
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
        request = service_pb2.Message(task="new_execution")
        request.data.update(self._algorithm.model_dump())
        sock.send(request.SerializeToString())
        rsp = service_pb2.Message()
        rsp.ParseFromString(sock.recv())
        if rsp.HasField("error"):
            raise Exception(rsp.error)
        return entity.backtest.Execution.parse_obj({k: v for k, v in rsp.data.items()})

    def _run_execution(self):
        req = service_pb2.Message(task="run_execution")
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
        request = service_pb2.Message(task="configure_execution")
        request.data.update(execution.model_dump())
        sock.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(sock.recv())
        if response.HasField("error"):
            raise Exception(response.error)
        instance = entity.service.Instance.parse_obj({k: v for k, v in response.data.items()})
        self._configure_execution(instance)
        self._run_execution()
        request = service_pb2.Message(task="run_execution")
        sock.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(sock.recv())
        if response.HasField("error"):
            raise Exception(response.error)
        import time

        time.sleep(2)
        while True:
            request = service_pb2.Message(task="current_period")
            sock.send(request.SerializeToString())
            response = service_pb2.Message()
            response.ParseFromString(sock.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            if not response.HasField("data"):
                break
            self.logger.info("current period: %s", response.data["timestamp"])

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
                    req = service_pb2.Message()
                    req.ParseFromString(b)
                except Exception as e:
                    self.logger.warning("Error deserializing request: %s", repr(e))
                    continue
                self.logger.info("received request: %s", req)
                response = service_pb2.Message(task=req.task)
                match req.task:
                    case "info":
                        response.data.update(self._algorithm.model_dump())
                        ctx.send(response.SerializeToString())
                    case "configure_execution":
                        self._configure_execution(
                            entity.service.Instance.parse_obj({k: v for k, v in req.data.items()})
                        )
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


class Foreverbull:
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
