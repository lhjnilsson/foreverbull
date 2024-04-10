import logging
from random import choice

import foreverbull
from foreverbull.data import Asset
from foreverbull.entity.finance import Order, Portfolio

logger = logging.getLogger(__name__)


@foreverbull.algo
def monkey(asset: Asset, portfolio: Portfolio) -> Order:
    return choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10), None])
