import enum
import socket
from datetime import datetime
from typing import List

import pydantic

from .finance import Portfolio


class SocketConfig(pydantic.BaseModel):
    class SocketType(str, enum.Enum):
        REQUESTER = "REQUESTER"
        REPLIER = "REPLIER"
        PUBLISHER = "PUBLISHER"
        SUBSCRIBER = "SUBSCRIBER"

    socket_type: SocketType = SocketType.REPLIER
    host: str = socket.gethostbyname(socket.gethostname())
    port: int = 0
    listen: bool = True
    recv_timeout: int = 20000
    send_timeout: int = 20000


class Service(pydantic.BaseModel):
    class Algorithm(pydantic.BaseModel):
        class Namespace(pydantic.BaseModel):
            type: str
            value_type: str

        class Function(pydantic.BaseModel):
            class Parameter(pydantic.BaseModel):
                key: str
                default: str | None = None
                type: str

            name: str
            parameters: List[Parameter]
            parallel_execution: bool
            run_first: bool
            run_last: bool

        file_path: str
        functions: list[Function]
        namespace: dict[str, Namespace] = {}

    class Status(pydantic.BaseModel):
        class Type(str, enum.Enum):
            CREATED = "CREATED"
            INTERVIEW = "INTERVIEW"
            READY = "READY"
            ERROR = "ERROR"

        status: Type
        error: str | None = None
        occurred_at: datetime

    image: str
    algorithm: Algorithm | None = None

    statuses: List[Status]


class Instance(pydantic.BaseModel):
    class Parameter(pydantic.BaseModel):
        parameters: dict[str, str]

    class Status(pydantic.BaseModel):
        class Type(str, enum.Enum):
            CREATED = "CREATED"
            RUNNING = "RUNNING"
            STOPPED = "STOPPED"
            ERROR = "ERROR"

        status: Type
        error: str | None = None
        occurred_at: datetime

    id: str
    host: str | None = None
    port: int | None = None
    broker_port: int | None = None
    namespace_port: int | None = None
    database_url: str | None = None
    functions: dict[str, Parameter] | None = None

    statuses: List[Status] | None = []
