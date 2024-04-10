import enum
import importlib
import types
import typing
from inspect import _empty, getabsfile, signature

from pydantic import BaseModel, ConfigDict, GetCoreSchemaHandler, TypeAdapter
from pydantic_core import CoreSchema, core_schema

from foreverbull.data import Asset
from foreverbull.entity.finance import Order, Portfolio


def type_to_str(type: any) -> str:
    match type():
        case int():
            return "int"
        case float():
            return "float"
        case bool():
            return "bool"
        case str():
            return "str"
        case _:
            raise Exception("Unknown parameter type: {}".format(type))


class Namespace(typing.Dict):
    def __init__(self, **kwargs):
        super().__init__()
        for key, value in kwargs.items():
            if type(value) != types.GenericAlias:
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

    def contains(self, key: str, type: types.GenericAlias) -> bool:
        if key not in self:
            raise KeyError("Key {} not found in namespace".format(key))
        if type.__origin__ == dict:
            if self[key]["type"] != "object":
                raise TypeError("Key {} is not of type object".format(key))
            if self[key]["items"]["type"] != type_to_str(type.__args__[1]):
                raise TypeError("Key {} is not of type {}".format(key, type))
        elif type.__origin__ == list:
            if self[key]["type"] != "array":
                raise TypeError("Key {} is not of type array".format(key))
            if self[key]["items"]["type"] != type_to_str(type.__args__[0]):
                raise TypeError("Key {} is not of type {}".format(key, type))
        else:
            raise TypeError("Unsupported namespace type")
        return True


class Function:
    def __init__(self, callable: callable, namespace_return_key: str | None = None, input_key: str = "symbols"):
        self.callable = callable
        self.namespace_return_key = namespace_return_key
        self.input_key = input_key


class Entity(BaseModel):
    model_config = ConfigDict(
        arbitrary_types_allowed=True,
    )

    class FunctionParameter(BaseModel):
        key: str
        default: str | None
        type: str

    class Style(enum.StrEnum):
        SEQUENTIAL = "SEQUENTIAL"
        PARALLEL = "PARALLEL"

    class ReturnType(enum.StrEnum):
        ORDER = "ORDER"
        LIST_OF_ORDERS = "LIST_OF_ORDERS"
        NAMESPACE_VALUE = "NAMESPACE_VALUE"

    class Function(BaseModel):
        name: str
        parameters: list["Entity.FunctionParameter"]
        style: "Entity.Style"
        return_type: "Entity.ReturnType"
        namespace_return_key: str | None = None

    file_path: str
    functions: list["Entity.Function"]
    namespace: Namespace


class Algorithm:
    _algo = None

    def __init__(self, functions: list[Function], namespace: Namespace = Namespace()):
        self._functions = functions
        self.namespace = namespace
        self.file_path = getabsfile(functions[0].callable)
        self.functions = []
        for f in functions:
            parameters = []
            style = None
            for key, value in signature(f.callable).parameters.items():
                if value.annotation == Portfolio:
                    continue
                if value.annotation == typing.List[Asset] or value.annotation == list[Asset]:
                    style = Entity.Style.SEQUENTIAL
                elif value.annotation == Asset:
                    style = Entity.Style.PARALLEL
                else:
                    default = None if value.default == value.empty else str(value.default)
                    parameter = Entity.FunctionParameter(key=key, default=default, type=type_to_str(value.annotation))
                    parameters.append(parameter)
            if style is None:
                raise Exception("No process style found for function {}".format(f.__name__))
            annotation = signature(f.callable).return_annotation
            if annotation == _empty:
                raise Exception("No return type found for function {}".format(f.__name__))
            if annotation == Order:
                return_type = Entity.ReturnType.ORDER
            elif annotation == typing.List[Order] or annotation == list[Order]:
                return_type = Entity.ReturnType.LIST_OF_ORDERS
            else:
                return_type = Entity.ReturnType.NAMESPACE_VALUE
                if f.namespace_return_key is None:
                    raise Exception("No namespace return key found for function {}".format(f.__name__))

            self.functions.append(
                Entity.Function(
                    name=f.callable.__name__,
                    parameters=parameters,
                    style=style,
                    return_type=return_type,
                )
            )
        self._entity = Entity(file_path=self.file_path, functions=self.functions, namespace=self.namespace)
        Algorithm._algo = self

    @classmethod
    def from_file_path(cls, file_path: str) -> "Algorithm":
        spec = importlib.util.spec_from_file_location("", file_path)
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)
        if Algorithm._algo is None:
            raise Exception("No algo found in {}".format(file_path))
        return Algorithm._algo

    @property
    def entity(self):
        return self._entity
