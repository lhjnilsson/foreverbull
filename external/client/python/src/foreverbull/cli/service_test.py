import traceback
from datetime import datetime
from unittest.mock import patch

from typer.testing import CliRunner

from foreverbull import entity
from foreverbull.cli.service import service

runner = CliRunner(mix_stderr=False)


def test_service_list():
    with patch("foreverbull.broker.service.list") as mock_list:
        mock_list.return_value = [
            entity.service.Service(
                name="test",
                image="test",
                statuses=[
                    entity.service.ServiceStatus(
                        status=entity.service.ServiceStatusType.READY,
                        error=None,
                        occurred_at=datetime.now(),
                    )
                ],
            )
        ]
        result = runner.invoke(service, ["list"])

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "test" in result.stdout
        assert "test" in result.stdout
        assert "READY" in result.stdout


def test_service_create():
    get_statuses = [
        entity.service.ServiceStatus(
            status=entity.service.ServiceStatusType.READY,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.service.ServiceStatus(
            status=entity.service.ServiceStatusType.INTERVIEW,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.service.ServiceStatus(
            status=entity.service.ServiceStatusType.CREATED,
            error=None,
            occurred_at=datetime.now(),
        ),
    ]

    with patch("foreverbull.broker.service.create") as mock_create, patch("foreverbull.broker.service.get") as mock_get:
        mock_create.return_value = entity.service.Service(
            name="test",
            image="test",
            statuses=get_statuses[2:],
        )
        mock_get.side_effect = [
            entity.service.Service(
                name="test",
                image="test",
                statuses=get_statuses[1:],
            ),
            entity.service.Service(
                name="test",
                image="test",
                statuses=get_statuses[1:],
            ),
            entity.service.Service(
                name="test",
                image="test",
                type="backtest_type",
                statuses=get_statuses,
            ),
        ]
        result = runner.invoke(service, ["create", "test", "test"])
        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "Service ready" in result.stdout
        assert "backtest_type" in result.stdout


def test_service_create_error():
    get_statuses = [
        entity.service.ServiceStatus(
            status=entity.service.ServiceStatusType.ERROR,
            error="error",
            occurred_at=datetime.now(),
        ),
        entity.service.ServiceStatus(
            status=entity.service.ServiceStatusType.INTERVIEW,
            error=None,
            occurred_at=datetime.now(),
        ),
        entity.service.ServiceStatus(
            status=entity.service.ServiceStatusType.CREATED,
            error=None,
            occurred_at=datetime.now(),
        ),
    ]

    with patch("foreverbull.broker.service.create") as mock_create, patch("foreverbull.broker.service.get") as mock_get:
        mock_create.return_value = entity.service.Service(
            name="test",
            image="test",
            statuses=get_statuses[2:],
        )
        mock_get.side_effect = [
            entity.service.Service(
                name="test",
                image="test",
                statuses=get_statuses[1:],
            ),
            entity.service.Service(
                name="test",
                image="test",
                statuses=get_statuses[1:],
            ),
            entity.service.Service(
                name="test",
                image="test",
                type="backtest_type",
                statuses=get_statuses,
            ),
        ]
        result = runner.invoke(service, ["create", "test", "test"])
        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert "Error while creating service: error" in result.stderr


def test_service_get():
    with (
        patch("foreverbull.broker.service.get") as mock_get,
        patch("foreverbull.broker.service.list_instances") as mock_list_instances,
    ):
        mock_get.return_value = entity.service.Service(
            name="test_name",
            image="test_image",
            type="backtest_type",
            statuses=[
                entity.service.ServiceStatus(
                    status=entity.service.ServiceStatusType.READY,
                    error=None,
                    occurred_at=datetime.now(),
                )
            ],
        )
        mock_list_instances.return_value = [
            entity.service.Instance(
                id="id123",
                service="test_name",
                host="hostname",
                port=1234,
                statuses=[
                    entity.service.InstanceStatus(
                        status=entity.service.InstanceStatusType.RUNNING,
                        error=None,
                        occurred_at=datetime.now(),
                    )
                ],
            )
        ]
        result = runner.invoke(service, ["get", "test"])

        if not result.exit_code == 0:
            traceback.print_exception(*result.exc_info)
        assert mock_get.called
        assert mock_list_instances.called
        assert "test_name" in result.stdout
        assert "test_image" in result.stdout
        assert "backtest_type" in result.stdout
        assert "id123" in result.stdout
        assert "hostname" in result.stdout
        assert "1234" in result.stdout
