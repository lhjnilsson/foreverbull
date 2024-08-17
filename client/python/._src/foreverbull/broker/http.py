import os
from functools import wraps
from typing import Callable, Concatenate

import requests


class RequestError(Exception):
    """Container of Exceptions from HTTP Client"""

    def __init__(self, response: requests.Response):
        self.response = response
        method = response.request.method
        url = response.request.url
        code = response.status_code
        text = response.text
        super().__init__(f"{method} call {url} gave bad return code: {code}. Text: {text}")


class Session(requests.Session):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.headers.update({"Content-Type": "application/json"})
        self._host = os.getenv("BROKER_HOSTNAME", "127.0.0.1")
        self._port = os.getenv("BROKER_HTTP_PORT", "8080")

    def request(self, method, url, *args, **kwargs) -> requests.Response:
        rsp = super().request(method, f"http://{self._host}:{self._port}{url}", **kwargs)
        if not rsp.ok:
            raise RequestError(rsp)
        return rsp


def inject_session[R, **P](f: Callable[Concatenate[Session, P], R]) -> Callable[P, R]:
    s = Session()

    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        return f(s, *args, **kwargs)

    return wrapper
