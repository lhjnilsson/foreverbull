import logging
import os
import signal
import socket

from foreverbull import broker

from . import execution

log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
logging.basicConfig(level=log_level)
log = logging.getLogger()

if __name__ == "__main__":
    execution = execution.Execution()
    execution.start()
    broker.service.update_instance(socket.gethostname(), True)
    log.info("starting application")
    signal.sigwait([signal.SIGTERM, signal.SIGINT])
    log.info("stopping application")
    execution.stop()
    broker.service.update_instance(socket.gethostname(), False)
    log.info("Exiting successfully")
