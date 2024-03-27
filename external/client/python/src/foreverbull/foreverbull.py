import logging
import os
import socket
import threading
from multiprocessing import Event

import pynng

from foreverbull import entity, import_file, worker


class BaseSession:
    def __init__(
        self,
        session: entity.backtest.Session,
        info: entity.service.Info,
        surveyor: pynng.Surveyor0,
        states: pynng.Sub0,
        workers: list[worker.Worker],
        stop_event: Event,
    ):
        self._session = session
        self._info = info
        self._surveyor = surveyor
        self._states = states
        self._workers = workers
        self._stop_event = stop_event
        self.logger = logging.getLogger(__name__)

    @property
    def session(self):
        return self._session

    @property
    def info(self):
        return self._info

    def configure_execution(self, execution: entity.backtest.Execution):
        self.logger.info("configuring workers")
        self._surveyor.send(entity.service.Request(task="configure_execution", data=execution.model_dump()).dump())
        responders = 0
        while True:
            try:
                rsp = entity.service.Response.load(self._surveyor.recv())
                if rsp.error:
                    raise worker.ConfigurationError(rsp.error)
                responders += 1
                self.logger.info("worker %s configured", responders)
                if responders == len(self._workers):
                    break
            except pynng.exceptions.Timeout:
                raise worker.ConfigurationError("Workers did not respond in time for configuration")
        self.logger.info("all workers configured")

    def run_execution(self):
        from uuid import uuid4

        self.logger.info(f"running backtest, {str(uuid4().hex)}")
        self._surveyor.send(entity.service.Request(task="run_execution").dump())
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


class ManualSession(BaseSession):
    def __init__(
        self,
        session: entity.backtest.Session,
        info: entity.service.Info,
        surveyor: pynng.Surveyor0,
        states: pynng.Sub0,
        workers: list[worker.Worker],
        stop_event: Event,
    ):
        BaseSession.__init__(self, session, info, surveyor, states, workers, stop_event)
        self.logger = logging.getLogger(__name__)

    def configure_execution(self, execution: entity.backtest.Execution):
        socket = pynng.Req0(
            dial=f"tcp://{os.getenv('BROKER_HOSTNAME', '127.0.0.1')}:{self._session.port}", block_on_dial=True
        )
        socket.send_timeout = 1000
        socket.recv_timeout = 1000
        socket.send(entity.service.Request(task="new_execution", data=execution).dump())
        rsp = entity.service.Response.load(socket.recv())
        if rsp.error:
            raise Exception(rsp.error)
        execution = entity.backtest.Execution(**rsp.data)
        return super().configure_execution(execution)

    def run_execution(self):
        super().run_execution()
        socket = pynng.Req0(
            dial=f"tcp://{os.getenv('BROKER_HOSTNAME', '127.0.0.1')}:{self._session.port}", block_on_dial=True
        )
        socket.send(entity.service.Request(task="run_execution").dump())
        rsp = entity.service.Response.load(socket.recv())
        if rsp.error:
            raise Exception(rsp.error)


class Session(threading.Thread, BaseSession):
    def __init__(
        self,
        session: entity.backtest.Session,
        info: entity.service.Info,
        surveyor: pynng.Surveyor0,
        states: pynng.Sub0,
        workers: list[worker.Worker],
        stop_event: Event,
    ):
        threading.Thread.__init__(self)
        BaseSession.__init__(self, session, info, surveyor, states, workers, stop_event)
        self.logger = logging.getLogger(__name__)
        self.socket_config = entity.service.SocketConfig(
            hostname=socket.gethostbyname(socket.gethostname()),
            port=5555,
            socket_type=entity.service.SocketType.REPLIER,
            listen=True,
        )

    def run(self):
        socket = pynng.Rep0(listen="tcp://0.0.0.0:5555")
        socket.recv_timeout = 300
        while not self._stop_event.is_set():
            ctx = socket.new_context()
            try:
                try:
                    b = ctx.recv()
                except Exception:
                    continue
                try:
                    req = entity.service.Request.load(b)
                except Exception:
                    # lib.nng_msg_free(self._nng_msg) seems to not work properly
                    # TODO FIX, maybe upstream in pynng
                    continue
                self.logger.info("received request: %s", req)
                match req.task:
                    case "info":
                        ctx.send(entity.service.Response(task="info", data=self.info).dump())
                    case "configure_execution":
                        data = self.configure_execution(entity.backtest.Execution(**req.data))
                        ctx.send(entity.service.Response(task="configure_execution", data=data).dump())
                    case "run_execution":
                        data = self.run_execution()
                        ctx.send(entity.service.Response(task="run_execution", data=data).dump())
                    case "stop":
                        ctx.send(entity.service.Response(task="stop").dump())
                        break
            except pynng.exceptions.Timeout:
                pass
            except Exception as e:
                self.logger.error("Error in socket runner: %s", repr(e))
            finally:
                ctx.close()
        socket.close()


class Foreverbull:
    def __init__(self, session: entity.backtest.Session, file_path: str = None, executors=2):
        self._session = session
        self._threaded_session = None
        self._file_path = file_path
        if self._file_path:
            try:
                import_file(self._file_path)
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
        algo = import_file(self._file_path)
        info = entity.service.Info(version="0.0.1", parameters=algo["parameters"])
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
                    algo["file_path"],
                )
            else:
                w = worker.WorkerProcess(
                    self._worker_surveyor_address,
                    self._worker_states_address,
                    self._stop_event,
                    algo["file_path"],
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
        if self._session.manual:
            return ManualSession(
                self._session,
                info,
                self._worker_surveyor_socket,
                self._worker_states_socket,
                self._workers,
                self._stop_event,
            )
        else:
            s = Session(
                self._session,
                info,
                self._worker_surveyor_socket,
                self._worker_states_socket,
                self._workers,
                self._stop_event,
            )
            s.start()
            self._threaded_session = s
            return s

    def __exit__(self, exc_type, exc_val, exc_tb):
        if not self._stop_event.is_set():
            self._stop_event.set()
        [worker.join() for worker in self._workers]
        self.logger.info("workers stopped")
        self._worker_surveyor_socket.close()
        self._worker_states_socket.close()
        self._stop_event = None
        if self._threaded_session:
            self._threaded_session.join()
            self._threaded_session = None
