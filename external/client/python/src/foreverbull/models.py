import importlib
import types
import typing
from functools import partial
from inspect import _empty, getabsfile, signature

from foreverbull import entity
from foreverbull.data import Asset


def type_to_str(
    type: any,
) -> str:
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
    def __init__(
        self,
        **kwargs,
    ):
        super().__init__()
        for (
            key,
            value,
        ) in kwargs.items():
            if not isinstance(
                value,
                types.GenericAlias,
            ):
                raise TypeError("Namespace values must be type annotations")
            if value.__origin__ == dict:
                self[key] = {"type": "object"}
                self[key]["items"] = {}
                if value.__args__[0] != str:
                    raise TypeError("Namespace keys must be strings")
                self[key]["items"]["type"] = type_to_str(value.__args__[1])
            elif value.__origin__ == list:
                self[key] = {"type": "array"}
                self[key]["items"] = {}
                self[key]["items"]["type"] = type_to_str(value.__args__[0])
            else:
                raise TypeError("Unsupported namespace type")
        return

    def contains(
        self,
        key: str,
        type: types.GenericAlias,
    ) -> bool:
        if key not in self:
            raise KeyError("Key {} not found in namespace".format(key))
        if type.__origin__ == dict:
            if self[key]["type"] != "object":
                raise TypeError("Key {} is not of type object".format(key))
            if self[key]["items"]["type"] != type_to_str(type.__args__[1]):
                raise TypeError(
                    "Key {} is not of type {}".format(
                        key,
                        type,
                    )
                )
        elif type.__origin__ == list:
            if self[key]["type"] != "array":
                raise TypeError("Key {} is not of type array".format(key))
            if self[key]["items"]["type"] != type_to_str(type.__args__[0]):
                raise TypeError(
                    "Key {} is not of type {}".format(
                        key,
                        type,
                    )
                )
        else:
            raise TypeError("Unsupported namespace type")
        return True


class Function:
    def __init__(
        self,
        callable: callable,
        namespace_return_key: str | None = None,
        input_key: str = "symbols",
    ):
        self.callable = callable
        self.namespace_return_key = namespace_return_key
        self.input_key = input_key


class Algorithm:
    _algo = None
    _file_path: str | None = None
    _functions: dict | None = None
    _namespace: Namespace | None = None

    def __init__(
        self,
        functions: list[Function],
        namespace: Namespace = dict(),
    ):
        Algorithm._file_path = getabsfile(functions[0].callable)
        Algorithm._functions = {}
        Algorithm._namespace = namespace

        for f in functions:
            parameters = []
            asset_key = None
            portfolio_key = None

            for (
                key,
                value,
            ) in signature(f.callable).parameters.items():
                if value.annotation == entity.finance.Portfolio:
                    portfolio_key = key
                    continue
                if value.annotation == typing.List[Asset] or value.annotation == list[Asset]:
                    parallel_execution = False
                    asset_key = key
                elif value.annotation == Asset:
                    parallel_execution = True
                    asset_key = key
                else:
                    default = None if value.default == value.empty else str(value.default)
                    parameter = entity.service.Algorithm.FunctionParameter(
                        key=key,
                        default=default,
                        type=type_to_str(value.annotation),
                    )
                    parameters.append(parameter)
            annotation = signature(f.callable).return_annotation
            if annotation == _empty:
                raise Exception("No return type found for function {}".format(f.__name__))
            if annotation == entity.finance.Order:
                return_type = entity.service.Algorithm.ReturnType.ORDER
            elif annotation == typing.List[entity.finance.Order] or annotation == list[entity.finance.Order]:
                return_type = entity.service.Algorithm.ReturnType.LIST_OF_ORDERS
            else:
                return_type = entity.service.Algorithm.ReturnType.NAMESPACE_VALUE
                if f.namespace_return_key is None:
                    raise Exception("No namespace return key found for function {}".format(f.__name__))

            function = {
                "callable": f.callable,
                "asset_key": asset_key,
                "portfolio_key": portfolio_key,
                "entity": entity.service.Algorithm.Function(
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

    def get_entity(
        self,
    ):
        e = entity.service.Algorithm(
            file_path=Algorithm._file_path,
            functions=[function["entity"] for function in Algorithm._functions.values()],
            namespace=Algorithm._namespace,
        )
        print("entity: ", e.model_dump_json())
        return e

    @classmethod
    def from_file_path(
        cls,
        file_path: str,
    ) -> "Algorithm":
        spec = importlib.util.spec_from_file_location(
            "",
            file_path,
        )
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)
        if Algorithm._algo is None:
            raise Exception("No algo found in {}".format(file_path))
        return Algorithm._algo

    def configure(
        self,
        execution: entity.service.Execution,
    ) -> None:
        def _eval_param(
            type: str,
            val,
        ):
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

        for (
            function_name,
            function,
        ) in Algorithm._functions.items():
            configuration = execution.configuration.get(function_name)

            for parameter in function["entity"].parameters:
                value = _eval_param(
                    "int",
                    configuration.parameters.get(parameter.key),
                )
                Algorithm._functions[function_name]["callable"] = partial(
                    function["callable"],
                    **{parameter.key: value},
                )

    def process(
        self,
        function_name: str,
        a: list[Asset] | Asset,
        portfolio: entity.finance.Portfolio,
    ) -> entity.finance.Order | list[entity.finance.Order] | dict:
        if Algorithm._functions[function_name]["portfolio_key"] is not None:
            return Algorithm._functions[function_name]["callable"](
                **{
                    Algorithm._functions[function_name]["portfolio_key"]: portfolio,
                    Algorithm._functions[function_name]["asset_key"]: a,
                }
            )
        return Algorithm._functions[function_name]["callable"](**{Algorithm._functions[function_name]["asset_key"]: a})
