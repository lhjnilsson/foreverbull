import logging
import os
import threading
from multiprocessing import Event

import pynng

from foreverbull import Algorithm, entity, socket, worker

from .exceptions import ConfigurationError


class Session(threading.Thread):
    def __init__(
        self,
        algorithm: entity.service.Service.Algorithm,
        surveyor: pynng.Surveyor0,
        states: pynng.Sub0,
        workers: list[worker.Worker],
        stop_event: Event,
    ):
        self._algorithm = algorithm
        self._surveyor = surveyor
        self._states = states
        self._workers = workers
        self._stop_event = stop_event
        self.logger = logging.getLogger(__name__)
        threading.Thread.__init__(self)

    @property
    def service(self):
        return self._service

    def _configure_execution(self, instance: entity.service.Instance):
        self.logger.info("configuring workers")
        self._surveyor.send(socket.Request(task="configure_execution", data=instance.model_dump()).serialize())
        responders = 0
        while True:
            try:
                rsp = socket.Response.deserialize(self._surveyor.recv())
                if rsp.error:
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
        sock.send(socket.Request(task="new_execution", data=self._algorithm).serialize())
        rsp = socket.Response.deserialize(sock.recv())
        if rsp.error:
            raise Exception(rsp.error)
        return entity.backtest.Execution(**rsp.data)

    def _run_execution(self):
        self._surveyor.send(socket.Request(task="run_execution").serialize())
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
        sock.send(socket.Request(task="configure_execution", data=execution).serialize())
        rsp = socket.Response.deserialize(sock.recv())
        if rsp.error:
            raise Exception(rsp.error)
        instance = entity.service.Instance(**rsp.data)
        self._configure_execution(instance)
        self._run_execution()
        sock.send(socket.Request(task="run_execution").serialize())
        rsp = socket.Response.deserialize(sock.recv())
        if rsp.error:
            raise Exception(rsp.error)

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
                    req = socket.Request.deserialize(b)
                except Exception as e:
                    self.logger.warning("Error deserializing request: %s", repr(e))
                    continue
                self.logger.info("received request: %s", req)
                match req.task:
                    case "info":
                        ctx.send(socket.Response(task="info", data=self._algorithm).serialize())
                    case "configure_execution":
                        data = self._configure_execution(entity.service.Instance(**req.data))
                        ctx.send(socket.Response(task="configure_execution", data=data).serialize())
                    case "run_execution":
                        data = self._run_execution()
                        ctx.send(socket.Response(task="run_execution", data=data).serialize())
                    case "stop":
                        ctx.send(socket.Response(task="stop").serialize())
                        break
                    case _:
                        ctx.send(socket.Response(task=req.task, error="Unknown task").serialize())
            except pynng.exceptions.Timeout:
                pass
            except Exception as e:
                self.logger.error("Error in socket runner: %s", repr(e))
            finally:
                ctx.close()
        sock.close()


class Foreverbull:
    def __init__(self, file_path: str = None, executors=2):
        self._session = None
        self._file_path = file_path
        if self._file_path:
            try:
                Algorithm.from_file_path(self._file_path)
            except Exception as e:
                raise ImportError(f"Could not import file {file_path}: {repr(e)}")
        self._executors = executors

        self._worker_surveyor_address = "ipc:///tmp/worker_pool.ipc"
        self._worker_surveyor_socket: pynng.Surveyor0 = None
        self._worker_states_address = "ipc:///tmp/worker_states.ipc"
        self._worker_states_socket: pynng.Sub0 = None
        self._stop_event: Event = None
        self._workers = []
        self.logger = logging.getLogger(__name__)

    def __enter__(self) -> Session:
        if self._file_path is None:
            raise Exception("No algo file provided")
        algo = Algorithm.from_file_path(self._file_path)
        self._worker_surveyor_socket = pynng.Surveyor0(listen=self._worker_surveyor_address)
        self._worker_surveyor_socket.sendout = 30000
        self._worker_surveyor_socket.recv_timeout = 30000
        self._worker_states_socket = pynng.Sub0(listen=self._worker_states_address)
        self._worker_states_socket.subscribe(b"")
        self._worker_states_socket.recv_timeout = 30000
        self._stop_event = Event()
        self.logger.info("starting workers")
        for i in range(self._executors):
            self.logger.info("starting worker %s", i)
            if os.getenv("THREADED_EXECUTION"):
                w = worker.WorkerThread(
                    self._worker_surveyor_address,
                    self._worker_states_address,
                    self._stop_event,
                    algo.get_entity().file_path,
                )
            else:
                w = worker.WorkerProcess(
                    self._worker_surveyor_address,
                    self._worker_states_address,
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
        if not self._stop_event.is_set():
            self._stop_event.set()
        [worker.join() for worker in self._workers]
        self.logger.info("workers stopped")
        self._worker_surveyor_socket.close()
        self._worker_states_socket.close()
        self._stop_event = None
        self._session.join()
        self._session = None
