import logging
import os

from foreverbull import entity
from foreverbull._version import version
from foreverbull.data import Asset
from foreverbull.entity.finance import Portfolio
from foreverbull.foreverbull import Foreverbull
from foreverbull.models import Algorithm, Function, Namespace

log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
logging.basicConfig(level=log_level)


__all__ = [Foreverbull, Asset, Portfolio, Algorithm, Function, Namespace, version, entity]
