import builtins
import importlib.util

from collections.abc import Callable
from functools import partial
from inspect import getabsfile
from inspect import isfunction
from inspect import signature

from foreverbull.exceptions import ConfigurationError
from foreverbull.models.finance import Asset
from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.service.v1 import worker_pb


class Function:
    def __init__(self, callable: Callable, run_first: bool = False, run_last: bool = False):
        self.callable = callable
        self.run_first = run_first
        self.run_last = run_last


_TRUE_VALUES = frozenset({"true", "1", "yes", "y", "on"})
_FALSE_VALUES = frozenset({"false", "0", "no", "n", "off"})


def _parse_bool(val: str) -> bool:
    normalized = val.strip().lower()
    if normalized in _TRUE_VALUES:
        return True
    if normalized in _FALSE_VALUES:
        return False
    raise ValueError(f"Invalid bool parameter value: {val!r}")


def _eval_param(type: str, val: str):
    if type == "int":
        return int(val)
    elif type == "float":
        return float(val)
    elif type == "bool":
        return _parse_bool(val)
    elif type in ("string", "str"):
        return val
    else:
        raise TypeError(f"Unknown parameter type: {type}")


class Algorithm:
    def __init__(self, functions: list[Function], namespaces: list[str] | None = None):
        self._file_path = getabsfile(functions[0].callable)
        self._functions = {}
        self._namespaces = [] if namespaces is None else namespaces

        for f in functions:
            self._functions[f.callable.__name__] = self._function_entry(f)

    def _function_entry(self, f: Function) -> dict:
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
                parameters.append(
                    worker_pb.Algorithm.FunctionParameter(
                        key=key,
                        defaultValue=default,
                        valueType=self.type_to_str(value.annotation),
                    )
                )
        if parallel_execution is None:
            raise TypeError(f"Function {f.callable.__name__} must have a parameter of type Asset or Assets")
        return {
            "callable": f.callable,
            "asset_key": asset_key,
            "portfolio_key": portfolio_key,
            "definition": worker_pb.Algorithm.Function(
                name=f.callable.__name__,  # type: ignore
                parameters=parameters,
                parallelExecution=parallel_execution,
                runFirst=f.run_first,
                runLast=f.run_last,
            ),
        }

    def get_definition(self) -> worker_pb.Algorithm:
        functions: list[worker_pb.Algorithm.Function] = []
        for function in self._functions.values():
            functions.append(function["definition"])
        return worker_pb.Algorithm(
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
            raise ConfigurationError(f"No spec found in {file_path}")
        if spec.loader is None:
            raise ConfigurationError(f"No loader found in {file_path}")
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)

        declared_variables = {
            name: obj for name, obj in source.__dict__.items() if not name.startswith("__") and not isfunction(obj)
        }
        for obj in declared_variables.values():
            if isinstance(obj, Algorithm):
                return obj
        raise ConfigurationError(f"No algorithm found in {file_path}")

    def _parameter_type(self, function_name: str, param_key: str) -> str:
        for f in self._functions.values():
            if f["definition"].name != function_name:
                continue
            for p in f["definition"].parameters:
                if p.key == param_key:
                    if not p.valueType:
                        raise TypeError("unable to determine parameter type")
                    return p.valueType
            raise TypeError(f"Unknown parameter: {param_key}")
        return ""

    def configure(self, function_name: str, param_key: str, param_value: str) -> None:
        value = _eval_param(self._parameter_type(function_name, param_key), param_value)
        function = self._functions[function_name]
        self._functions[function_name]["callable"] = partial(
            function["callable"],
            **{param_key: value},
        )

    def process(
        self,
        function_name: str,
        portfolio: marketdata_pb.Portfolio,
        symbols: list[str],
        bars: list[marketdata_pb.OHLC] | None = None,
    ) -> list[marketdata_pb.Order]:
        function = self._functions[function_name]
        delivered = [] if bars is None else list(bars)
        p = Portfolio(portfolio, delivered)
        as_of = pb_utils.from_proto_timestamp(portfolio.timestamp)
        if function["definition"].parallelExecution:
            for symbol in symbols:
                asset = Asset(as_of, symbol, delivered)
                function["callable"](
                    asset=asset,
                    portfolio=p,
                )
        else:
            assets = Assets(as_of, symbols, delivered)
            function["callable"](assets=assets, portfolio=p)
        return p.pending_orders
