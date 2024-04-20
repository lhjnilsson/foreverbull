from unittest.mock import patch

import pytest
import requests
from pydantic import BaseModel

from foreverbull.broker.http import RequestError, api_call


def test_api_call_with_response_model():
    class ResponseModel(BaseModel):
        test: str

    @api_call(ResponseModel)
    def test_func():
        return requests.Request(
            "GET",
            "/test",
        )

    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = {"test": "test"}
        assert test_func() == ResponseModel(test="test")
        mock_send.assert_called_once()


def test_api_call_without_response_model():
    @api_call()
    def test_func():
        return requests.Request(
            "GET",
            "/test",
        )

    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = True
        mock_send.return_value.json.return_value = {"test": "test"}
        assert test_func() == {"test": "test"}
        mock_send.assert_called_once()


def test_api_call_error():
    @api_call()
    def test_func():
        return requests.Request(
            "GET",
            "/test",
        )

    with patch("requests.Session.send") as mock_send:
        mock_send.return_value.ok = False
        mock_send.return_value.status_code = 500
        mock_send.return_value.text = "error"
        with pytest.raises(RequestError):
            test_func()
