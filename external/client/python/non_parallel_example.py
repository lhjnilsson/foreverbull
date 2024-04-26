import logging
from random import choice

import foreverbull
from foreverbull.data import Assets
from foreverbull.entity.finance import Order, Portfolio

logger = logging.getLogger(__name__)


@foreverbull.algo
def monkey(assets: Assets, portfolio: Portfolio):
    orders = []
    for asset in assets:
        order = choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10), None])
        if order:
            orders.append(order)
    return orders
