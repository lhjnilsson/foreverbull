import builtins
import importlib.util

from functools import partial
from inspect import getabsfile
from inspect import signature
from typing import Callable

from sqlalchemy import Connection

from foreverbull.models.finance import Asset
from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2


class Function:
    def __init__(self, callable: Callable, run_first: bool = False, run_last: bool = False):
        self.callable = callable
        self.run_first = run_first
        self.run_last = run_last


class Algorithm:
    def __init__(self, functions: list[Function], namespaces: list[str] = []):
        self._algo = None
        self._file_path = getabsfile(functions[0].callable)
        self._functions = {}
        self._namespaces = namespaces

        for f in functions:
            parameters = []
            asset_key = None
            portfolio_key = None
            parallel_execution: bool | None = None

            for key, value in signature(f.callable).parameters.items():
                if value.annotation == Portfolio:
                    portfolio_key = key
                    continue
                if issubclass(value.annotation, Assets):
                    parallel_execution = False
                    asset_key = key
                elif issubclass(value.annotation, Asset):
                    parallel_execution = True
                    asset_key = key
                else:
                    default = None if value.default == value.empty else str(value.default)
                    parameter = worker_pb2.Algorithm.FunctionParameter(
                        key=key,
                        defaultValue=default,
                        valueType=self.type_to_str(value.annotation),
                    )
                    parameters.append(parameter)
            if parallel_execution is None:
                raise TypeError("Function {} must have a parameter of type Asset or Assets".format(f.callable.__name__))
            function = {
                "callable": f.callable,
                "asset_key": asset_key,
                "portfolio_key": portfolio_key,
                "definition": worker_pb2.Algorithm.Function(
                    name=f.callable.__name__,  # type: ignore
                    parameters=parameters,
                    parallelExecution=parallel_execution,
                    runFirst=f.run_first,
                    runLast=f.run_last,
                ),
            }

            self._functions[f.callable.__name__] = function

    def get_definition(self) -> worker_pb2.Algorithm:
        functions: list[worker_pb2.Algorithm.Function] = []
        for name, function in self._functions.items():
            functions.append(function["definition"])
        return worker_pb2.Algorithm(
            file_path=self._file_path,
            functions=functions,
            namespaces=self._namespaces,
        )

    @staticmethod
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
        import inspect

        declared_variables = {
            name: obj
            for name, obj in source.__dict__.items()
            if not name.startswith("__") and not inspect.isfunction(obj)
        }
        for name, obj in declared_variables.items():
            if isinstance(obj, Algorithm):
                return obj
        raise Exception("No algorithm found in {}".format(file_path))

    def configure(self, function_name: str, param_key: str, param_value: str) -> None:
        def _eval_param(type: str, val: str):
            if type == "int":
                return int(val)
            elif type == "float":
                return float(val)
            elif type == "bool":
                return bool(val)
            elif type == "str":
                return val
            else:
                raise TypeError(f"Unknown parameter type: {type}")

        param_type: str = ""
        for f in self._functions.values():
            if f["definition"].name == function_name:
                function_name = f["definition"].name
                for p in f["definition"].parameters:
                    if p.key == param_key:
                        if not p.valueType:
                            raise TypeError("unable to determine parameter type")
                        param_type = p.valueType
                        break
                else:
                    raise TypeError(f"Unknown parameter: {param_key}")
                break

        value = _eval_param(param_type, param_value)
        function = self._functions[function_name]
        self._functions[function_name]["callable"] = partial(
            function["callable"],
            **{param_key: value},
        )

    def process(
        self,
        function_name: str,
        db: Connection,
        portfolio: marketdata_pb2.Portfolio,
        symbols: list[str],
    ) -> list[marketdata_pb2.Order]:
        p = Portfolio(portfolio, db)
        if self._functions[function_name]["definition"].parallelExecution:
            for symbol in symbols:
                asset = Asset(pb_utils.from_proto_timestamp(portfolio.timestamp), db, symbol)
                self._functions[function_name]["callable"](
                    asset=asset,
                    portfolio=p,
                )
        else:
            assets = Assets(pb_utils.from_proto_timestamp(portfolio.timestamp), db, symbols)
            self._functions[function_name]["callable"](assets=assets, portfolio=p)
        return p.pending_orders
