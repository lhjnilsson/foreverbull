import logging
import os
import signal

from . import service


log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
logging.basicConfig(level=log_level)
log = logging.getLogger()

if __name__ == "__main__":
    log.info("Starting foreverbull_zipline")
    with service.grpc_server() as server:
        log.info("starting grpc server")
        signal.sigwait([signal.SIGTERM, signal.SIGINT])
        log.info("stopping grpc server")
        server.stop(None)
        log.info("stopping engine")
    log.info("exiting")
