import importlib
import types
import typing
from functools import partial
from inspect import _empty, getabsfile, signature

from foreverbull import entity
from foreverbull.data import Asset, Assets


def type_to_str(type: any) -> str:
    match type():
        case int():
            return "int"
        case float():
            return "float"
        case bool():
            return "bool"
        case str():
            return "string"
        case _:
            raise Exception("Unknown parameter type: {}".format(type))


class Namespace(typing.Dict):
    def __init__(self, **kwargs):
        super().__init__()
        for key, value in kwargs.items():
            if not isinstance(value, types.GenericAlias):
                raise TypeError("Namespace values must be type annotations")
            if value.__origin__ == dict:
                self[key] = {"type": "object"}
                self[key]["value_type"] = type_to_str(value.__args__[1])
            elif value.__origin__ == list:
                self[key] = {"type": "array"}
                self[key]["value_type"] = type_to_str(value.__args__[0])
            else:
                raise TypeError("Unsupported namespace type")
        return

    def contains(self, key: str, type: types.GenericAlias) -> bool:
        if key not in self:
            raise KeyError("Key {} not found in namespace".format(key))
        if type.__origin__ == dict:
            if self[key]["type"] != "object":
                raise TypeError("Key {} is not of type object".format(key))
            if self[key]["value_type"] != type_to_str(type.__args__[1]):
                raise TypeError("Key {} is not of type {}".format(key, type))
        elif type.__origin__ == list:
            if self[key]["type"] != "array":
                raise TypeError("Key {} is not of type array".format(key))
            if self[key]["value_type"] != type_to_str(type.__args__[0]):
                raise TypeError("Key {} is not of type {}".format(key, type))
        else:
            raise TypeError("Unsupported namespace type")
        return True


class Function:
    def __init__(self, callable: callable, namespace_return_key: str | None = None, input_key: str = "symbols"):
        self.callable = callable
        self.namespace_return_key = namespace_return_key
        self.input_key = input_key


class Algorithm:
    _algo = None
    _file_path: str | None = None
    _functions: dict | None = None
    _namespace: Namespace | None = None

    def __init__(self, functions: list[Function], namespace: Namespace = dict()):
        Algorithm._file_path = getabsfile(functions[0].callable)
        Algorithm._functions = {}
        Algorithm._namespace = Namespace(**namespace)

        for f in functions:
            parameters = []
            asset_key = None
            portfolio_key = None

            for key, value in signature(f.callable).parameters.items():
                if value.annotation == entity.finance.Portfolio:
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
            annotation = signature(f.callable).return_annotation
            if annotation == _empty:
                raise Exception("No return type found for function {}".format(f.__name__))

            if annotation == entity.finance.Order:
                return_type = entity.service.Service.Algorithm.Function.ReturnType.ORDER
            elif annotation == typing.List[entity.finance.Order] or annotation == list[entity.finance.Order]:
                return_type = entity.service.Service.Algorithm.Function.ReturnType.LIST_OF_ORDERS
            else:
                return_type = entity.service.Service.Algorithm.Function.ReturnType.NAMESPACE_VALUE
                if f.namespace_return_key is None:
                    raise Exception("No namespace return key found for function {}".format(f.__name__))

            function = {
                "callable": f.callable,
                "asset_key": asset_key,
                "portfolio_key": portfolio_key,
                "entity": entity.service.Service.Algorithm.Function(
                    name=f.callable.__name__,
                    parameters=parameters,
                    parallel_execution=parallel_execution,
                    return_type=return_type,
                    input_key=f.input_key,
                    namespace_return_key=f.namespace_return_key,
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
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)
        if Algorithm._algo is None:
            raise Exception("No algo found in {}".format(file_path))
        return Algorithm._algo

    def configure(self, parameters: dict[str, entity.service.Instance.Parameter]) -> None:
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

        for function_name, function in Algorithm._functions.items():
            configuration = parameters.get(function_name)

            for parameter in function["entity"].parameters:
                value = _eval_param("int", configuration.parameters.get(parameter.key))
                Algorithm._functions[function_name]["callable"] = partial(
                    function["callable"],
                    **{parameter.key: value},
                )

    def process(
        self,
        function_name: str,
        db: any,
        request: entity.service.Request,
    ) -> list[entity.finance.Order]:
        if Algorithm._functions[function_name]["entity"].parallel_execution:
            orders = []
            for symbol in request.symbols:
                a = Asset(request.timestamp, None, symbol)
                order = Algorithm._functions[function_name]["callable"](
                    asset=a,
                    portfolio=request.portfolio,
                )
                if order:
                    orders.append(order)
        else:
            assets = Assets(request.timestamp, db, request.symbols)
            orders = Algorithm._functions[function_name]["callable"](assets=assets, portfolio=request.portfolio)
        return orders
