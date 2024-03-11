from unittest.mock import patch

import pytest

from foreverbull.broker import service
from foreverbull.entity.service import Instance, Service


@pytest.mark.parametrize(
    "return_value, expected_model",
    [
        ([], []),
    ],
)
def test_service_list(return_value, expected_model):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert service.list() == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_image",
            {
                "image": "test_image",
                "instances": [],
                "statuses": [],
            },
            Service(image="test_image", instances=[], statuses=[]),
        ),
    ],
)
def test_service_create(argument, return_value, expected_model):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert service.create(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            "test_image",
            {
                "image": "test_image",
                "instances": [],
                "statuses": [],
            },
            Service(image="test_image", instances=[], statuses=[]),
        ),
    ],
)
def test_service_get(argument, return_value, expected_model):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert service.get(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        ("test_image", [], []),
    ],
)
def test_service_list_instances(argument, return_value, expected_model):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert service.list_instances(argument) == expected_model
        mock_send.assert_called_once()


@pytest.mark.parametrize(
    "argument, return_value, expected_model",
    [
        (
            ("test_container", None),
            {
                "id": "test_container",
                "image": "test_image",
                "statuses": [],
            },
            Instance(id="test_container", image="test_image", statuses=[]),
        ),
    ],
)
def test_service_update_instance(argument, return_value, expected_model):
    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = return_value
        assert service.update_instance(*argument) == expected_model
        mock_send.assert_called_once()
