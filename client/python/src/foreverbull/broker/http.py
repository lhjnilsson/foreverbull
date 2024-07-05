import os
from typing import Generic, TypeVar

import requests
from pydantic import BaseModel

host = os.getenv("BROKER_HOSTNAME", "127.0.0.1")
port = os.getenv("BROKER_HTTP_PORT", "8080")


class RequestError(Exception):
    """Container of Exceptions from HTTP Client"""

    def __init__(self, request: requests.Request, response: requests.Response):
        self.request = request
        self.response = response
        method = request.method
        url = request.url
        code = response.status_code
        text = response.text
        super().__init__(f"{method} call {url} gave bad return code: {code}. Text: {text}")


def api_call(response_model: BaseModel | None = None):
    s = requests.Session()

    def wrapper(func):
        def make_request(*args, **kwargs):
            req: requests.Request = func(*args, **kwargs)
            req.url = f"http://{host}:{port}{req.url}"
            rsp = s.send(req.prepare())
            if not rsp.ok:
                raise RequestError(req, rsp)
            if response_model:
                if isinstance(rsp.json(), list):
                    return [response_model.model_validate(**x) for x in rsp.json()]
                return response_model.model_validate(rsp.json())
            return rsp.json()

        return make_request

    return wrapper
