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
def update_instance(container_id: str, socket: entity.service.SocketConfig = None) -> requests.Request:
    return requests.Request(
        method="PATCH",
        url=f"/service/api/instances/{container_id}",
        json={**socket.model_dump()} if socket else {},
    )
