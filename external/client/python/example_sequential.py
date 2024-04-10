import logging
from random import choice
from typing import List

import foreverbull
from foreverbull.data import Asset
from foreverbull.entity.finance import Order, Portfolio

logger = logging.getLogger(__name__)


@foreverbull.algo
def monkey(assets: List[Asset], portfolio: Portfolio) -> List[Order]:
    orders = []
    for asset in assets:
        order = choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10), None])
        if order:
            orders.append(order)
    return orders
