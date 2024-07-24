import logging
import logging.handlers
import os
from multiprocessing import Process, Queue
from multiprocessing.synchronize import Event
from threading import Thread

import pynng
from sqlalchemy import text

from foreverbull import Algorithm, entity, exceptions
from foreverbull.data import get_engine
from foreverbull.pb_gen import finance_pb2, service_pb2


class Worker:
    def __init__(
        self, survey_address: str, state_address: str, logging_queue: Queue, stop_event: Event, file_path: str
    ):
        self._survey_address = survey_address
        self._state_address = state_address
        self._logging_queue = logging_queue
        self._stop_event = stop_event
        self._database = None
        self._file_path = file_path
        self._parallel = False
        super(Worker, self).__init__()

    def configure_execution(self, instance: entity.service.Instance):
        self.logger.info("configuring worker")
        self._algo = Algorithm.from_file_path(self._file_path)
        try:
            self.socket = pynng.Rep0(
                dial=f"tcp://{os.getenv('BROKER_HOSTNAME', '127.0.0.1')}:{instance.broker_port}", block_on_dial=True
            )
            self.socket.recv_timeout = 5000
            self.socket.send_timeout = 5000
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to broker: {e}")

        try:
            self._algo.configure(instance.functions if instance.functions else {})
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to setup algorithm: {e}")

        if instance.database_url is None:
            raise exceptions.ConfigurationError("Database URL is not set")

        try:
            engine = get_engine(instance.database_url)
            with engine.connect() as connection:
                connection.execute(text("SELECT 1 from asset;"))
            self._database_engine = engine
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to database: {e}")

        os.environ["NAMESPACE_PORT"] = str(instance.namespace_port)
        self.logger.info("worker configured correctly")

    def run(self):
        if self._logging_queue:
            handler = logging.handlers.QueueHandler(self._logging_queue)
            logging.basicConfig(level=logging.DEBUG, handlers=[handler])
        self.logger = logging.getLogger(__name__)
        try:
            responder = pynng.Respondent0(dial=self._survey_address, block_on_dial=True)
            responder.send_timeout = 5000
            responder.recv_timeout = 300
            state = pynng.Pub0(dial=self._state_address, block_on_dial=True)
            state.send(b"ready")
        except Exception as e:
            self.logger.error("Unable to connect to surveyor or state sockets")
            self.logger.exception(repr(e))
            return 1

        self.logger.info("starting worker")
        while not self._stop_event.is_set():
            request = service_pb2.Message()
            try:
                request.ParseFromString(responder.recv())
                self.logger.info(f"Received request: {request.task}")
                if request.task == "configure_execution":
                    instance = entity.service.Instance.model_validate(request.data)
                    self.configure_execution(instance)
                    response = service_pb2.Message(task=request.task, error=None)
                    responder.send(response.SerializeToString())
                elif request.task == "run_execution":
                    response = service_pb2.Message(task=request.task, error=None)
                    responder.send(response.SerializeToString())
                    self.run_execution()
            except pynng.exceptions.Timeout:
                self.logger.debug("Timeout in pynng while running, continuing...")
                continue
            except Exception as e:
                self.logger.error("Error processing request")
                self.logger.exception(repr(e))
                response = service_pb2.Message(task=request.task, error=repr(e))
                responder.send(response.SerializeToString())
            self.logger.info(f"Request processed: {request.task}")
        responder.close()
        state.close()

    def run_execution(self):
        while True:
            request = None
            self.logger.debug("Getting context socket")
            context_socket = self.socket.new_context()
            try:
                request = service_pb2.Request()
                request.ParseFromString(context_socket.recv())
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
                    request.orders.append(finance_pb2.Order(**order.model_dump()))
                context_socket.send(request.SerializeToString())
                context_socket.close()
            except pynng.exceptions.Timeout:
                context_socket.close()
            except Exception as e:
                self.logger.exception(repr(e))
                if request:
                    request.error = repr(e)
                    context_socket.send(request.SerializeToString())
                if context_socket:
                    context_socket.close()
            if self._stop_event.is_set():
                break
        self.socket.close()


class WorkerThread(Worker, Thread):  # type: ignore
    pass


class WorkerProcess(Worker, Process):  # type: ignore
    pass
