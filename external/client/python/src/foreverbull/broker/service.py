import socket as _socket

import requests

from foreverbull import entity

from .http import api_call


@api_call(response_model=entity.service.Service)
def list() -> requests.Request:
    return requests.Request(
        method="GET",
        url="/service/api/services",
    )


@api_call(response_model=entity.service.Service)
def create(image: str) -> requests.Request:
    return requests.Request(
        method="POST",
        url="/service/api/services",
        json={"image": image},
    )


@api_call(response_model=entity.service.Service)
def get(image: str) -> requests.Request:
    return requests.Request(
        method="GET",
        url=f"/service/api/services/{image}",
    )


@api_call(response_model=entity.service.Instance)
def list_instances(image: str = None) -> requests.Request:
    return requests.Request(
        method="GET",
        url="/service/api/instances",
        params={"image": image},
    )


@api_call(response_model=entity.service.Instance)
def update_instance(container_id: str, online: bool) -> requests.Request:
    if online:
        socket_config = entity.service.SocketConfig(
            hostname=_socket.gethostbyname(_socket.gethostname()),
            port=5555,
            socket_type=entity.service.SocketConfig.SocketType.REPLIER,
            listen=True,
        )
    else:
        socket_config = None
    return requests.Request(
        method="PATCH",
        url=f"/service/api/instances/{container_id}",
        json={**socket_config.model_dump()} if socket_config else {},
    )
