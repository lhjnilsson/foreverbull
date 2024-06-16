import logging
import os
from multiprocessing import set_start_method

from foreverbull.cli import cli

if __name__ == "__main__":
    set_start_method("spawn")
    log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
    logging.basicConfig(level=log_level)
    cli()
