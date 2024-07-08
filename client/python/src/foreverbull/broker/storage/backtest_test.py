import io
from unittest.mock import Mock

import numpy as np
import pytest
import urllib3
from pandas import DataFrame

from foreverbull.broker.storage.backtest import Backtest


@pytest.fixture
def mocked_backtest():
    return Backtest(Mock())


def test_backtest_list_backtest_results(mocked_backtest: Backtest):
    mocked_backtest.client.list_objects = Mock(return_value=[])

    assert mocked_backtest.list_backtest_results() == []
    mocked_backtest.client.list_objects.assert_called_once_with("backtest-results")


def test_backtest_upload_backtest_result(mocked_backtest: Backtest):
    mocked_backtest.client.fput_object = Mock()

    mocked_backtest.upload_backtest_result("backtest", Mock())
    mocked_backtest.client.fput_object.assert_called_once_with("backtest-results", "backtest", "/tmp/result.pkl")


def test_backtest_download_backtest_results(mocked_backtest: Backtest):
    buffer = io.BytesIO()
    df = DataFrame(np.random.randint(0, 100, size=(100, 4)))
    df.to_pickle(buffer)
    buffer.seek(0)
    urllib3.BaseHTTPResponse = Mock()
    urllib3.BaseHTTPResponse.read = Mock(return_value=buffer.getvalue())
    mocked_backtest.client.get_object = Mock(return_value=urllib3.BaseHTTPResponse)

    assert df.equals(mocked_backtest.download_backtest_results("backtest"))
    mocked_backtest.client.get_object.assert_called_once_with("backtest-results", "backtest")


def test_backtest_list_backtest_ingestions(mocked_backtest: Backtest):
    mocked_backtest.client.list_objects = Mock(return_value=[])

    assert mocked_backtest.list_backtest_ingestions() == []
    mocked_backtest.client.list_objects.assert_called_once_with("backtest-ingestions")


def test_backtest_upload_backtest_ingestion(mocked_backtest: Backtest):
    mocked_backtest.client.fput_object = Mock()

    mocked_backtest.upload_backtest_ingestion("local", "remote")
    mocked_backtest.client.fput_object.assert_called_once_with("backtest-ingestions", "remote", "local")


def test_backtest_download_backtest_ingestion(mocked_backtest: Backtest):
    mocked_backtest.client.fget_object = Mock()

    mocked_backtest.download_backtest_ingestion("remote", "local")
    mocked_backtest.client.fget_object.assert_called_once_with("backtest-ingestions", "remote", "local")
