import logging
from random import choice

import foreverbull
from foreverbull.data import Asset
from foreverbull.entity.finance import Order, Portfolio

logger = logging.getLogger(__name__)


@foreverbull.algo
def monkey(asset: Asset, portfolio: Portfolio):
    return choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10), None])


def filter_assets(assets: list[Asset]) -> list[Asset]:
    pass


def measure_asset(asset: Asset) -> float:
    pass


def place_orders(assets: list[Asset], portfolio: Portfolio) -> list[Order]:
    pass


class Example:
    def __init__(self):
        pass

    def __setattr__(self, __name: str, __value: any) -> None:
        print(__name, __value)
        print("YOO")

    def __getattribute__(self, __name: str) -> any:
        print(__name)
        print("YOO")
        return "hehe"


from typing import List

from pydantic import BaseModel


class NamespaceMetrics(BaseModel):
    key: str
    value: int


class Namespace(BaseModel):
    filtered_symbols: List[str]
    asset_metrics: dict[str, NamespaceMetrics]


class AlgorithmFunction:
    def __init__(self, function):
        self.function = function
        self.output = None


class Algorithm:
    def __init__(self, functions: AlgorithmFunction):
        self.functions = functions
        self.namespace = {}


algo = Algorithm(
    functions=[AlgorithmFunction(filter_assets), AlgorithmFunction(measure_asset), AlgorithmFunction(place_orders)]
)
