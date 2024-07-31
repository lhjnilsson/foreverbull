import builtins
import importlib.util
import types
import typing
from datetime import datetime
from functools import partial
from inspect import getabsfile, signature
from typing import Any, Callable

from sqlalchemy import Connection

from foreverbull import entity
from foreverbull.data import Asset, Assets, Portfolio
from foreverbull.pb.finance import finance_pb2


def type_to_str[T: (int, float, bool, str)](t: T) -> str:
    match t:
        case builtins.int:
            return "int"
        case builtins.float:
            return "float"
        case builtins.bool:
            return "bool"
        case builtins.str:
            return "string"
        case _:
            raise TypeError("Unsupported type: ", type(t))


class Namespace(typing.Dict):
    def __init__(self, **kwargs):
        super().__init__()
        for key, value in kwargs.items():
            if not isinstance(value, types.GenericAlias):
                raise TypeError("Namespace values must be type annotations")
            if value.__origin__ is dict:
                self[key] = {"type": "object"}
                self[key]["value_type"] = type_to_str(value.__args__[1])
            elif value.__origin__ is list:
                self[key] = {"type": "array"}
                self[key]["value_type"] = type_to_str(value.__args__[0])
            else:
                raise TypeError("Unsupported namespace type")
        return

    def contains(self, key: str, type: Any) -> bool:
        if key not in self:
            raise KeyError("Key {} not found in namespace".format(key))
        if type.__origin__ is dict:
            if self[key]["type"] != "object":
                raise TypeError("Key {} is not of type object".format(key))
            if self[key]["value_type"] is not type_to_str(type.__args__[1]):
                raise TypeError("Key {} is not of type {}".format(key, type))
        elif type.__origin__ is list:
            if self[key]["type"] != "array":
                raise TypeError("Key {} is not of type array".format(key))
            if self[key]["value_type"] is not type_to_str(type.__args__[0]):
                raise TypeError("Key {} is not of type {}".format(key, type))
        else:
            raise TypeError("Unsupported namespace type")
        return True


class Function:
    def __init__(self, callable: Callable, run_first: bool = False, run_last: bool = False):
        self.callable = callable
        self.run_first = run_first
        self.run_last = run_last


class Algorithm:
    _algo: "Algorithm | None"
    _file_path: str
    _functions: dict
    _namespace: Namespace

    def __init__(self, functions: list[Function], namespace: Namespace | dict = {}):
        Algorithm._algo = None
        Algorithm._file_path = getabsfile(functions[0].callable)
        Algorithm._functions = {}
        Algorithm._namespace = Namespace(**namespace)

        for f in functions:
            parameters = []
            asset_key = None
            portfolio_key = None
            parallel_execution: bool | None = None

            for key, value in signature(f.callable).parameters.items():
                if value.annotation == Portfolio:
                    portfolio_key = key
                    continue
                if value.annotation == Assets:
                    parallel_execution = False
                    asset_key = key
                elif value.annotation == Asset:
                    parallel_execution = True
                    asset_key = key
                else:
                    default = None if value.default == value.empty else str(value.default)
                    parameter = entity.service.Service.Algorithm.Function.Parameter(
                        key=key,
                        default=default,
                        type=type_to_str(value.annotation),
                    )
                    parameters.append(parameter)
            if parallel_execution is None:
                raise TypeError("Function {} must have a parameter of type Asset or Assets".format(f.callable.__name__))

            function = {
                "callable": f.callable,
                "asset_key": asset_key,
                "portfolio_key": portfolio_key,
                "entity": entity.service.Service.Algorithm.Function(
                    name=f.callable.__name__,
                    parameters=parameters,
                    parallel_execution=parallel_execution,
                    run_first=f.run_first,
                    run_last=f.run_last,
                ),
            }

            Algorithm._functions[f.callable.__name__] = function
        Algorithm._algo = self

    def get_entity(self):
        e = entity.service.Service.Algorithm(
            file_path=Algorithm._file_path,
            functions=[function["entity"] for function in Algorithm._functions.values()],
            namespace=Algorithm._namespace,
        )
        return e

    @classmethod
    def from_file_path(cls, file_path: str) -> "Algorithm":
        spec = importlib.util.spec_from_file_location(
            "",
            file_path,
        )
        if spec is None:
            raise Exception("No spec found in {}".format(file_path))
        if spec.loader is None:
            raise Exception("No loader found in {}".format(file_path))
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)
        if Algorithm._algo is None:
            raise Exception("No algo found in {}".format(file_path))
        return Algorithm._algo

    def configure(self, function_name: str, param_key: str, param_value: str, param_type: str) -> None:
        def _eval_param(type: str, val: str):
            if type == "int":
                return int(val)
            elif type == "float":
                return float(val)
            elif type == "bool":
                return bool(val)
            elif type == "str":
                return str(val)
            else:
                raise TypeError("Unknown parameter type")
        value = _eval_param(param_type, param_value)
        function = Algorithm._functions[function_name]
        Algorithm._functions[function_name]["callable"] = partial(
            function["callable"],
            **{param_key: value},
        )

    def process(
        self,
        function_name: str,
        db: Connection,
        portfolio: finance_pb2.Portfolio,
        timestamp: datetime,
        symbols: list[str],
    ) -> list[entity.finance.Order]:
        p = Portfolio(cash=portfolio.cash, value=portfolio.value,
            positions=[entity.finance.Position(symbol=p.symbol, amount=p.amount, cost_basis=p.cost) for p in portfolio.positions])
        if Algorithm._functions[function_name]["entity"].parallel_execution:
            orders = []
            for symbol in symbols:
                a = Asset(timestamp, db, symbol)
                order = Algorithm._functions[function_name]["callable"](
                    asset=a,
                    portfolio=p,
                )
                if order:
                    orders.append(order)
        else:
            assets = Assets(timestamp, db, symbols)
            orders = Algorithm._functions[function_name]["callable"](assets=assets, portfolio=p)
            if not orders:
                orders = []
        return orders
