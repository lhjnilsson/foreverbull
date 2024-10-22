import logging
from rich.live import Live


class LoggingHandler(logging.Handler):
    def __init__(self, live: Live):
        super().__init__()
        self.live = live
        self.log_buffer = []
        self.setLevel(logging.ERROR)
        self.setFormatter(logging.Formatter("%(message)s"))

    def emit(self, record: logging.LogRecord):
        log_entry = self.format(record)
        self.log_buffer.append(log_entry)
        for entry in self.log_buffer:
            self.live.console.print(entry)
