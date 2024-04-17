import enum
import socket
from datetime import datetime
from typing import Any, List, Optional

import pydantic
from pydantic import BaseModel, ConfigDict

from .base import Base


class Execution(Base):
    class Function(BaseModel):
        parameters: dict[str, str] | None = None

    id: str
    database_url: str
    port: int
    configuration: dict[str, "Execution.Function"]


class SocketType(str, enum.Enum):
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


class ServiceStatusType(str, enum.Enum):
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
        default: str | None
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
        namespace_return_key: str | None = None

    file_path: str
    functions: list["Algorithm.Function"]
    namespace: dict


class Service(Base):
    image: str
    algorithm: Algorithm | None = None

    statuses: List[ServiceStatus] = []


class InstanceStatusType(str, enum.Enum):
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
    def validate_data(cls, v):
        if v is None:
            return v
        if isinstance(v, dict):
            return v
        if isinstance(v, list):
            return v
        return v.model_dump()


class Response(Base):
    task: str
    error: Optional[str] = None
    data: Optional[Any] = None

    @pydantic.field_validator("data")
    def validate_data(cls, v):
        if v is None:
            return v
        if isinstance(v, dict):
            return v
        if isinstance(v, list):
            return v
        return v.model_dump()
