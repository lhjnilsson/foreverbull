import json

import pydantic


class Request(pydantic.BaseModel):
    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)
    task: str
    data: pydantic.SkipValidation[any] = None
    error: pydantic.SkipValidation[any] = None

    def serialize(self) -> bytes:
        return self.model_dump_json().encode()

    @classmethod
    def deserialize(cls, data: bytes) -> "Request":
        if isinstance(data, dict):
            return cls(**data)
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
    error: pydantic.SkipValidation[any] = None
    data: pydantic.SkipValidation[any] = None

    def serialize(self) -> bytes:
        return self.model_dump_json().encode()

    @classmethod
    def deserialize(cls, data: bytes) -> "Request":
        if isinstance(data, dict):
            return cls(**data)
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
