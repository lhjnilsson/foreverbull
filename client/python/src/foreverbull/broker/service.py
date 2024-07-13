import socket as _socket
from typing import List

from foreverbull import entity

from .http import Session, inject_session


@inject_session
def list(session: Session) -> List[entity.service.Service]:
    rsp = session.request("GET", "/service/api/services")
    return [entity.service.Service.parse_obj(s) for s in rsp.json()]


@inject_session
def create(session: Session, image: str) -> entity.service.Service:
    rsp = session.request("POST", "/service/api/services", json={"image": image})
    return entity.service.Service.parse_obj(rsp.json())


@inject_session
def get(session: Session, image: str) -> entity.service.Service:
    rsp = session.request("GET", f"/service/api/services/{image}")
    return entity.service.Service.parse_obj(rsp.json())


@inject_session
def list_instances(session: Session, image: str | None = None) -> List[entity.service.Instance]:
    rsp = session.request("GET", "/service/api/instances", params={"image": image})
    return [entity.service.Instance.parse_obj(i) for i in rsp.json()]


@inject_session
def update_instance(session: Session, container_id: str, online: bool) -> entity.service.Instance:
    if online:
        socket_config = entity.service.SocketConfig(
            host=_socket.gethostbyname(_socket.gethostname()),
            port=5555,
            socket_type=entity.service.SocketConfig.SocketType.REPLIER,
            listen=True,
        )
    else:
        socket_config = None

    rsp = session.request(
        "PATCH", f"/service/api/instances/{container_id}", json={**socket_config.model_dump()} if socket_config else {}
    )
    return entity.service.Instance.parse_obj(rsp.json())
