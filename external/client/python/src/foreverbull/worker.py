import logging
import os
from datetime import datetime
from multiprocessing import Event, Process
from threading import Thread
from typing import List

import pynng
from pydantic import BaseModel
from sqlalchemy import text

from . import entity, exceptions
from .data import Asset, get_engine
from .models import Algorithm


class Request(BaseModel):
    timestamp: datetime
    symbol: str | None = None
    symbols: List[str] | None = None
    portfolio: entity.finance.Portfolio


class Worker:
    def __init__(
        self,
        survey_address: str,
        state_address: str,
        stop_event: Event,
        file_path: str,
    ):
        self._survey_address = survey_address
        self._state_address = state_address
        self._stop_event = stop_event
        self._database = None
        self._file_path = file_path
        self._algo = None
        self.logger = logging.getLogger(__name__)
        super(
            Worker,
            self,
        ).__init__()

    @staticmethod
    def _eval_param(
        type: str,
        val,
    ):
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

    def configure_execution(
        self,
        execution: entity.service.Execution,
    ):
        self._algo = Algorithm.from_file_path(self._file_path)
        self.logger.info("configuring worker")
        try:
            self.socket = pynng.Rep0(
                dial=f"tcp://{os.getenv('BROKER_HOSTNAME', '127.0.0.1')}:{execution.port}",
                block_on_dial=True,
            )
            self.socket.recv_timeout = 5000
            self.socket.send_timeout = 5000
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to broker: {e}")

        try:
            self._algo.configure(execution)
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to setup algorithm: {e}")

        try:
            engine = get_engine(execution.database_url)
            with engine.connect() as connection:
                connection.execute(text("SELECT 1 from asset;"))
            self._database_engine = engine
        except Exception as e:
            raise exceptions.ConfigurationError(f"Unable to connect to database: {e}")
        self.logger.info("worker configured correctly")

    def run(
        self,
    ):
        try:
            responder = pynng.Respondent0(
                dial=self._survey_address,
                block_on_dial=True,
            )
            responder.send_timeout = 5000
            responder.recv_timeout = 300
            state = pynng.Pub0(
                dial=self._state_address,
                block_on_dial=True,
            )
            state.send(b"ready")
        except Exception as e:
            self.logger.error("Unable to connect to surveyor or state sockets")
            self.logger.exception(repr(e))
            return 1

        self.logger.info("starting worker")
        while not self._stop_event.is_set():
            try:
                request = entity.service.Request.load(responder.recv())
                self.logger.info(f"Received request: {request.task}")
                if request.task == "configure_execution":
                    execution = entity.service.Execution(**request.data)
                    self.configure_execution(execution)
                    responder.send(
                        entity.service.Response(
                            task=request.task,
                            error=None,
                        ).dump()
                    )
                elif request.task == "run_execution":
                    responder.send(
                        entity.service.Response(
                            task=request.task,
                            error=None,
                        ).dump()
                    )
                    self.run_execution()
            except pynng.exceptions.Timeout:
                self.logger.debug("Timeout in pynng while running, continuing...")
                continue
            except Exception as e:
                self.logger.error("Error processing request")
                self.logger.exception(repr(e))
                responder.send(
                    entity.service.Response(
                        task=request.task,
                        error=repr(e),
                    ).dump()
                )
            self.logger.info(f"Request processed: {request.task}")
        responder.close()
        state.close()

    def run_execution(
        self,
    ):
        while True:
            request = None
            context_socket = None
            try:
                self.logger.debug("Getting context socket")
                context_socket = self.socket.new_context()
                request = entity.service.Request.load(context_socket.recv())
                data = Request(**request.data)
                self.logger.debug(f"processing request: {data}")
                with self._database_engine.connect() as db:
                    if data.symbol:
                        ret = self._algo.process(
                            request.task,
                            a=Asset(
                                data.symbol,
                                data.timestamp,
                                db,
                            ),
                            portfolio=data.portfolio,
                        )
                    elif data.symbols:
                        assets = [
                            Asset(
                                symbol,
                                data.timestamp,
                                db,
                            )
                            for symbol in data.symbols
                        ]
                        ret = self._algo.process(
                            request.task,
                            a=assets,
                            portfolio=data.portfolio,
                        )
                self.logger.debug(f"Sending response {ret}")
                context_socket.send(
                    entity.service.Response(
                        task=request.task,
                        data=ret,
                    ).dump()
                )
                context_socket.close()
            except pynng.exceptions.Timeout:
                context_socket.close()
            except Exception as e:
                self.logger.exception(repr(e))
                if request:
                    context_socket.send(
                        entity.service.Response(
                            task=request.task,
                            error=repr(e),
                        ).dump()
                    )
                if context_socket:
                    context_socket.close()
            if self._stop_event.is_set():
                break
        self.socket.close()


class WorkerThread(
    Worker,
    Thread,
):
    pass


class WorkerProcess(
    Worker,
    Process,
):
    pass
