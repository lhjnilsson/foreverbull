import json
from typing import Union

from pydantic import BaseModel


class Base(BaseModel):
    @classmethod
    def load(cls, data: Union[dict, bytes]) -> object:
        if isinstance(data, dict):
            return cls(**data)
        loaded = json.loads(data.decode())
        return cls(**loaded)

    def dump(self) -> bytes:
        return self.model_dump_json().encode()
