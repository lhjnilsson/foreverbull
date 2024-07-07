import zipline
import zipline.errors
from zipline import TradingAlgorithm
from zipline.protocol import BarData

from . import entity


class BrokerError(Exception):
    pass


class Broker:
    def can_trade(self, asset: entity.Asset, trading_algorithm: TradingAlgorithm, data: BarData) -> bool:
        try:
            equity = trading_algorithm.symbol(asset.symbol)
        except zipline.errors.SymbolNotFound as e:
            raise BrokerError(repr(e))
        return data.can_trade(equity)

    def order(self, order: entity.Order, trading_algorithm: TradingAlgorithm) -> entity.Order:
        try:
            asset = trading_algorithm.symbol(order.symbol)
        except zipline.errors.SymbolNotFound as e:
            raise BrokerError(repr(e))
        order.id = trading_algorithm.order(
            asset=asset, amount=order.amount, limit_price=order.limit_price, stop_price=order.stop_price
        )
        return entity.Order.from_zipline(trading_algorithm.get_order(order.id))

    def get_order(self, order: entity.Order, trading_algorithm: TradingAlgorithm) -> entity.Order:
        event = trading_algorithm.get_order(order.id)
        if event is None:
            raise BrokerError(f"order {order.id} not found")
        order = entity.Order.from_zipline(event)
        return order

    def get_open_orders(self, trading_algorithm: TradingAlgorithm) -> list[entity.Order]:
        return [entity.Order.from_zipline(o) for _, o in trading_algorithm.get_open_orders().items()]

    def cancel_order(self, order: entity.Order, trading_algorithm: TradingAlgorithm) -> entity.Order:
        trading_algorithm.cancel_order(order.id)
        event = trading_algorithm.get_order(order.id)
        if event is None:
            raise BrokerError(f"order {order.id} not found")
        order = entity.Order.from_zipline(event)
        return order
