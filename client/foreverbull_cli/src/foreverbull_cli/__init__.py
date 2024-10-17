from multiprocessing import set_start_method


import logging
from rich.logging import RichHandler

FORMAT = "%(message)s"
logging.basicConfig(level="NOTSET", format=FORMAT, datefmt="[%X]", handlers=[RichHandler()])

try:
    set_start_method("spawn")
except RuntimeError:
    pass
