import json
from typing import Any

import pydantic


class Request(pydantic.BaseModel):
    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)
    task: str
    data: pydantic.SkipValidation[Any] = None
    error: pydantic.SkipValidation[Any] = None

    def serialize(self) -> bytes:
        return self.model_dump_json().encode()

    @classmethod
    def deserialize(cls, data: bytes | dict) -> "Request":
        if isinstance(data, dict):
            return cls(**data)
        elif isinstance(data, memoryview):
            return cls(**json.loads(data.tobytes().decode()))
        return cls(**json.loads(data.decode()))

    @pydantic.field_validator("data")
    def validate_data(cls, v):
        if v is None:
            return v
        if isinstance(v, dict):
            return v
        if isinstance(v, list):
            return v
        return v.model_dump()


class Response(pydantic.BaseModel):
    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)
    task: str
    error: pydantic.SkipValidation[Any] = None
    data: pydantic.SkipValidation[Any] = None

    def serialize(self) -> bytes:
        return self.model_dump_json().encode()

    @classmethod
    def deserialize(cls, data: bytes | dict) -> "Response":
        if isinstance(data, dict):
            return cls(**data)
        elif isinstance(data, memoryview):
            return cls(**json.loads(data.tobytes().decode()))
        return cls(**json.loads(data.decode()))

    @pydantic.field_validator("data")
    def validate_data(cls, v):
        if v is None:
            return v
        if isinstance(v, dict):
            return v
        if isinstance(v, list):
            return v
        return v.model_dump()
