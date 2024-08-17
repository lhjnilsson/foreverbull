import zipline
import zipline.errors
from zipline import TradingAlgorithm
from zipline.protocol import BarData


class BrokerError(Exception):
    pass


class Broker:
    def can_trade(self, asset, trading_algorithm: TradingAlgorithm, data: BarData) -> bool:
        try:
            equity = trading_algorithm.symbol(asset.symbol)
        except zipline.errors.SymbolNotFound as e:
            raise BrokerError(repr(e))
        return data.can_trade(equity)

    def order(self, symbol: str, amount: int, trading_algorithm: TradingAlgorithm):
        try:
            asset = trading_algorithm.symbol(symbol)
        except zipline.errors.SymbolNotFound as e:
            raise BrokerError(repr(e))
        order_id = trading_algorithm.order(asset=asset, amount=amount)
        return trading_algorithm.get_order(order_id)

    def get_order(self, order, trading_algorithm: TradingAlgorithm):
        return trading_algorithm.get_order(order.id)

    def get_open_orders(self, trading_algorithm: TradingAlgorithm):
        orders = []
        for _, open_orders in trading_algorithm.get_open_orders().items():
            for order in open_orders:
                orders.append(order)
        return orders

    def cancel_order(self, order, trading_algorithm: TradingAlgorithm):
        trading_algorithm.cancel_order(order.id)
        return trading_algorithm.get_order(order.id)
