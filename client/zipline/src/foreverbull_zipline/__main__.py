import logging
import os
import signal

from multiprocessing import get_start_method
from multiprocessing import set_start_method

from . import service


log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
logging.basicConfig(level=log_level)
log = logging.getLogger()

if __name__ == "__main__":
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)

    log.info("Starting foreverbull_zipline")
    with service.grpc_server() as server:
        log.info("starting grpc server")
        signal.sigwait([signal.SIGTERM, signal.SIGINT])
        log.info("stopping grpc server")
        server.stop(None)
        log.info("stopping engine")
    log.info("exiting")
