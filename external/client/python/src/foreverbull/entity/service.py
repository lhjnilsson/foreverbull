import enum
import socket
import types
from datetime import datetime
from typing import Any, List, Optional

import pydantic
from pydantic import BaseModel, ConfigDict

from .base import Base


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


class Execution(Base):
    class Function(BaseModel):
        parameters: (
            dict[
                str,
                str,
            ]
            | None
        ) = None

    id: str
    database_url: str
    port: int
    configuration: dict[
        str,
        "Execution.Function",
    ]


class SocketType(
    str,
    enum.Enum,
):
    REQUESTER = "REQUESTER"
    REPLIER = "REPLIER"
    PUBLISHER = "PUBLISHER"
    SUBSCRIBER = "SUBSCRIBER"


class SocketConfig(Base):
    socket_type: SocketType = SocketType.REPLIER
    host: str = socket.gethostbyname(socket.gethostname())
    port: int = 0
    listen: bool = True
    recv_timeout: int = 20000
    send_timeout: int = 20000


class ServiceStatusType(
    str,
    enum.Enum,
):
    CREATED = "CREATED"
    INTERVIEW = "INTERVIEW"
    READY = "READY"
    ERROR = "ERROR"


class ServiceStatus(Base):
    status: ServiceStatusType
    error: str | None = None
    occurred_at: datetime


class Algorithm(Base):
    model_config = ConfigDict(
        arbitrary_types_allowed=True,
    )

    class FunctionParameter(BaseModel):
        key: str
        default: str | None = None
        type: str

    class ReturnType(enum.StrEnum):
        ORDER = "ORDER"
        LIST_OF_ORDERS = "LIST_OF_ORDERS"
        NAMESPACE_VALUE = "NAMESPACE_VALUE"

    class Function(BaseModel):
        name: str
        parameters: list["Algorithm.FunctionParameter"]
        parallel_execution: bool = False
        return_type: "Algorithm.ReturnType"
        input_key: str = "symbols"
        namespace_return_key: str | None = None

    class Namespace(BaseModel):
        type: str
        value_type: str

    file_path: str
    functions: list["Algorithm.Function"]
    namespace: dict[
        str,
        Namespace,
    ] = {}

    @pydantic.field_validator(
        "namespace",
        mode="before",
    )
    @classmethod
    def validate_namespace(
        cls,
        v,
    ):
        if not isinstance(
            v,
            dict,
        ):
            raise ValueError("Namespace must be a dictionary")
        namespace = {}
        for (
            key,
            value,
        ) in v.items():
            if isinstance(
                value,
                Algorithm.Namespace,
            ):
                namespace[key] = value
                continue
            if not isinstance(
                value,
                types.GenericAlias,
            ):
                raise ValueError("Namespace value must be a GenericAlias")

            if value.__origin__ == dict:
                if len(value.__args__) != 2:
                    raise ValueError(
                        "Expected typed dict with 2 arguments, has: ",
                        len(value.__args__),
                    )
                if value.__args__[0] != str:
                    raise ValueError(
                        "Expected typed dict with string keys, has: ",
                        value.__args__[0],
                    )

                namespace[key] = Algorithm.Namespace(
                    type="object",
                    value_type=type_to_str(value.__args__[1]),
                )
            elif value.__origin__ == list:
                namespace[key] = Algorithm.Namespace(
                    type="array",
                    value_type=type_to_str(value.__args__[0]),
                )
            else:
                raise ValueError("Unsupported namespace type")
        return namespace


class Service(Base):
    image: str
    algorithm: Algorithm | None = None

    statuses: List[ServiceStatus] = []


class InstanceStatusType(
    str,
    enum.Enum,
):
    CREATED = "CREATED"
    RUNNING = "RUNNING"
    STOPPED = "STOPPED"
    ERROR = "ERROR"


class InstanceStatus(Base):
    status: InstanceStatusType
    error: str | None = None
    occurred_at: datetime


class Instance(Base):
    id: str
    image: str
    host: str | None = None
    port: int | None = None

    statuses: List[InstanceStatus]


class Request(Base):
    task: str
    data: Optional[Any] = None
    error: Optional[str] = None

    @pydantic.field_validator("data")
    @classmethod
    def validate_data(
        cls,
        v,
    ):
        if v is None:
            return v
        if isinstance(
            v,
            dict,
        ):
            return v
        if isinstance(
            v,
            list,
        ):
            return v
        return v.model_dump()


class Response(Base):
    task: str
    error: Optional[str] = None
    data: Optional[Any] = None

    @pydantic.field_validator("data")
    @classmethod
    def validate_data(
        cls,
        v,
    ):
        if v is None:
            return v
        if isinstance(
            v,
            dict,
        ):
            return v
        if isinstance(
            v,
            list,
        ):
            return v
        return v.model_dump()
