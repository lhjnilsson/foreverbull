import logging
import os
import signal
import socket

from foreverbull import broker

from . import engine, grpc_servicer

log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
logging.basicConfig(level=log_level)
log = logging.getLogger()

if __name__ == "__main__":
    log.info("Starting foreverbull_zipline")
    engine = engine.EngineProcess()
    engine.start()
    engine.is_ready.wait(3.0)
    with grpc_servicer.grpc_server(engine) as server:
        log.info("starting grpc server")
        signal.sigwait([signal.SIGTERM, signal.SIGINT])
        log.info("stopping grpc server")
        server.stop(None)
        log.info("stopping engine")
    engine.stop()
    engine.join(3.0)
    log.info("exiting")
