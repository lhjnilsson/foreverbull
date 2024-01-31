import logging
import signal
import socket

from foreverbull import broker

from .execution import Execution

log = logging.getLogger()

if __name__ == "__main__":
    execution = Execution()
    execution.start()
    broker.service.update_instance(socket.gethostname(), execution.socket_config)
    log.info("starting application")
    signal.sigwait([signal.SIGTERM, signal.SIGINT])
    log.info("stopping application")
    execution.stop()
    broker.service.update_instance(socket.gethostname(), None)
    log.info("Exiting successfully")
