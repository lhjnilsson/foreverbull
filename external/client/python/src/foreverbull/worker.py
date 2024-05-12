import logging
import os
from multiprocessing import Event, Process
from threading import Thread

import pynng
from sqlalchemy import text

from foreverbull import Algorithm, entity, exceptions, socket
from foreverbull.data import get_engine


class Worker:
    def __init__(self, survey_address: str, state_address: str, stop_event: Event, file_path: str):
        self._survey_address = survey_address
        self._state_address = state_address
        self._stop_event = stop_event
        self._database = None
        self._file_path = file_path
        self._parallel = False
        self.logger = logging.getLogger(__name__)
        super(Worker, self).__init__()

    @staticmethod
    def _eval_param(type: str, val):
        if type == "int":
            return int(val)
        elif type == "float":
            return float(val)
        elif type == "bool":
            return bool(val)
        elif type == "str":
            return str(val)
        else:
            raise TypeError("Unknown parameter type")

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
            self._algo.configure(instance.functions)
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to setup algorithm: {e}")

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
            try:
                request = socket.Request.deserialize(responder.recv())
                self.logger.info(f"Received request: {request.task}")
                if request.task == "configure_execution":
                    instance = entity.service.Instance(**request.data)
                    self.configure_execution(instance)
                    responder.send(socket.Response(task=request.task, error=None).serialize())
                elif request.task == "run_execution":
                    responder.send(socket.Response(task=request.task, error=None).serialize())
                    self.run_execution()
            except pynng.exceptions.Timeout:
                self.logger.debug("Timeout in pynng while running, continuing...")
                continue
            except Exception as e:
                self.logger.error("Error processing request")
                self.logger.exception(repr(e))
                responder.send(socket.Response(task=request.task, error=repr(e)).serialize())
            self.logger.info(f"Request processed: {request.task}")
        responder.close()
        state.close()

    def run_execution(self):
        while True:
            request = None
            context_socket = None
            try:
                self.logger.debug("Getting context socket")
                context_socket = self.socket.new_context()
                request = socket.Request.deserialize(context_socket.recv())
                data = entity.service.Request(**request.data)
                self.logger.debug(f"processing request: {data}")
                with self._database_engine.connect() as db:
                    orders = self._algo.process(request.task, db, data)
                self.logger.debug(f"Sending response {orders}")
                context_socket.send(socket.Response(task=request.task, data=orders).serialize())
                context_socket.close()
            except pynng.exceptions.Timeout:
                context_socket.close()
            except Exception as e:
                self.logger.exception(repr(e))
                if request:
                    context_socket.send(socket.Response(task=request.task, error=repr(e)).serialize())
                if context_socket:
                    context_socket.close()
            if self._stop_event.is_set():
                break
        self.socket.close()


class WorkerThread(Worker, Thread):
    pass


class WorkerProcess(Worker, Process):
    pass
