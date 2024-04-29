import enum
from datetime import datetime, timezone
from typing import List, Optional

import pydantic

from .base import Base


class BacktestStatusType(str, enum.Enum):
    CREATED = "CREATED"
    UPDATED = "UPDATED"
    INGESTING = "INGESTING"
    READY = "READY"
    ERROR = "ERROR"


class BacktestStatus(pydantic.BaseModel):
    status: BacktestStatusType
    error: str | None = None
    occurred_at: datetime


class Backtest(pydantic.BaseModel):
    name: str
    service: Optional[str] = None
    calendar: str = "XNYS"
    start: datetime
    end: datetime
    benchmark: str | None = None
    symbols: List[str]

    data_frequency: str = "daily"
    capital_base: int = 100_000

    statuses: List[BacktestStatus] | None = None

    sessions: int | None = None

    @pydantic.field_serializer("start")
    def start_iso(self, start: datetime, _info):
        if start.tzinfo is None:
            start = start.replace(tzinfo=timezone.utc)
        return start.strftime("%Y-%m-%dT%H:%M:%SZ")

    @pydantic.field_serializer("end")
    def end_iso(self, end: datetime, _info):
        if end.tzinfo is None:
            end = end.replace(tzinfo=timezone.utc)
        return end.strftime("%Y-%m-%dT%H:%M:%SZ")


class SessionStatusType(str, enum.Enum):
    CREATED = "CREATED"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class SessionStatus(pydantic.BaseModel):
    status: SessionStatusType
    error: str | None = None
    occurred_at: datetime


class Session(pydantic.BaseModel):
    id: Optional[str] = None
    backtest: str
    manual: bool = False
    executions: int

    statuses: List[SessionStatus] = []

    port: int | None = None


class ExecutionStatusType(str, enum.Enum):
    CREATED = "CREATED"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class ExecutionStatus(pydantic.BaseModel):
    status: ExecutionStatusType
    error: str | None = None
    occurred_at: datetime


class Execution(pydantic.BaseModel):
    id: Optional[str] = None
    calendar: str = "XNYS"
    start: Optional[datetime] = None
    end: Optional[datetime] = None
    benchmark: Optional[str] = None
    symbols: Optional[List[str]] = None
    capital_base: int = 100000
    database: Optional[str] = None

    statuses: List[ExecutionStatus] = []

    port: int | None = None

    @pydantic.field_serializer("start")
    def start_iso(self, start: datetime, _info):
        if start is None:
            return None

        if start.tzinfo is None:
            start = start.replace(tzinfo=timezone.utc)
        return start.isoformat()

    @pydantic.field_serializer("end")
    def end_iso(self, end: datetime, _info):
        if end is None:
            return None

        if end.tzinfo is None:
            end = end.replace(tzinfo=timezone.utc)
        return end.isoformat()
