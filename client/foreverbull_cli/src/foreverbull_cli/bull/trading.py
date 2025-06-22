import logging

from typing import List

import typer

from rich.live import Live
from rich.progress import Progress
from rich.table import Table
from typing_extensions import Annotated

from foreverbull import broker
from foreverbull import models
from foreverbull.foreverbull import Foreverbull
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull_cli.output import FBProgress
from foreverbull_cli.output import console


trading = typer.Typer()
log = logging.getLogger().getChild(__name__)


class PositionAndOrder:
    def __init__(self, position: marketdata_pb2.Position | None, order: marketdata_pb2.Order | None):
        if position is None and order is None:
            raise ValueError("Both position and order are None")
        self.position = position
        self.order = order

    @classmethod
    def with_defaults(
        cls, position: marketdata_pb2.Position | None, order: marketdata_pb2.Order | None
    ) -> "PositionAndOrder":
        if order and position is None:
            if order.amount > 0:
                order.intent = marketdata_pb2.PositionIntent.POSITION_INTENT_BUY_TO_OPEN
            else:
                order.intent = marketdata_pb2.PositionIntent.POSITION_INTENT_SELL_TO_OPEN

        elif order and position:
            if position.amount > 0 and order.amount + position.amount == 0:
                order.intent = marketdata_pb2.PositionIntent.POSITION_INTENT_SELL_TO_CLOSE
            elif position.amount < 0 and order.amount + position.amount == 0:
                order.intent = marketdata_pb2.PositionIntent.POSITION_INTENT_BUY_TO_CLOSE

        if order:
            order.type = marketdata_pb2.OrderType.ORDER_TYPE_MARKET
            order.time_in_force = marketdata_pb2.TimeInForce.TIME_IN_FORCE_DAY
        return PositionAndOrder(position, order)

    @classmethod
    def parse(
        cls, positions: list[marketdata_pb2.Position], orders: list[marketdata_pb2.Order]
    ) -> List["PositionAndOrder"]:
        items = []
        for position in positions:
            for order in orders:
                if position.symbol == order.symbol:
                    items.append(PositionAndOrder.with_defaults(position, order))
                    break
            else:
                items.append(PositionAndOrder(position, None))
        for order in orders:
            if order.symbol not in [item.symbol for item in items]:
                items.append(PositionAndOrder.with_defaults(None, order))
        return items

    @property
    def symbol(self) -> str:
        return self.position.symbol if self.position is not None else self.order.symbol  # type: ignore

    @property
    def amount(self) -> str:
        current_position = self.position.amount if self.position is not None else 0
        order_amount = self.order.amount if self.order is not None else 0
        if order_amount:
            return f"{current_position} -> {current_position + order_amount}"
        return str(current_position)

    @property
    def cost_basis(self) -> str:
        return str(self.position.cost_basis) if self.position is not None else "-"

    @property
    def order_type(self) -> str:
        return str(self.order.type) if self.order is not None else "-"

    @property
    def time_in_force(self) -> str:
        return str(self.order.time_in_force) if self.order is not None else "-"

    @property
    def position_intent(self) -> str:
        return str(self.order.intent) if self.order is not None else "-"


@trading.command()
def run(
    backtest_name: Annotated[str, typer.Argument(help="name of the backtest which symbols to trade")],
    file_path: Annotated[str, typer.Argument(help="path to the trading strategy file")],
    dry: Annotated[bool, typer.Option(help="dry run the strategy")] = False,
    yes: Annotated[bool, typer.Option(help="skip confirmation prompt")] = False,
):
    _backtest = broker.backtest.get(backtest_name)
    symbols = [s for s in _backtest.symbols]
    portfolio = broker.finance.get_portfolio()

    with FBProgress() as progress:
        setup_task = progress.add_task("Setting Up", total=2)
        run_task = progress.add_task("Running Trading", total=2)
        progress.update(setup_task, completed=1)
        with Foreverbull(models.Algorithm.from_file_path(file_path)).trading_session(symbols) as session:
            progress.update(setup_task, completed=2)
            progress.update(run_task, completed=1)
            orders, place_orders = session.run_trading(symbols)
            progress.update(run_task, completed=2)

            items = PositionAndOrder.parse([p for p in portfolio.portfolio.positions], orders)
            orders_table = Table(title="Positions and Orders")
            orders_table.add_column("Symbol")
            orders_table.add_column("Amount")
            orders_table.add_column("Cost Basis")
            orders_table.add_column("Order Type")
            orders_table.add_column("Time In Force")
            orders_table.add_column("Position Intent")
            for item in items:
                orders_table.add_row(
                    item.symbol, item.amount, item.cost_basis, item.order_type, item.time_in_force, item.position_intent
                )

            # Clean up terminal
            progress.remove_task(setup_task)
            progress.remove_task(run_task)

            console.print(orders_table)

            progress.stop()
            place_order = typer.confirm("Place order?")
            progress.start()

            if place_order:
                place_orders_task = progress.add_task("Placing Orders", total=2)
                progress.update(place_orders_task, completed=1)
                place_orders(orders)
                progress.update(place_orders_task, completed=2)


@trading.command()
def demo():
    live = Live(Progress())
    with live:
        c = typer.confirm("Continue?")
        if not c:
            return
        print("Running demo")
