from unittest.mock import patch

import pytest

from foreverbull.broker.storage.storage import Storage


@pytest.mark.parametrize(
    "env",
    [
        {},
        {"STORAGE_ENDPOINT": "localhost:9000"},
        {"STORAGE_ENDPOINT": "localhost:9000", "STORAGE_ACCESS_KEY": "minioadmin"},
        {
            "STORAGE_ENDPOINT": "localhost:9000",
            "STORAGE_ACCESS_KEY": "minioadmin",
            "STORAGE_SECRET_KEY": "minioadmin",
        },
        {
            "STORAGE_ENDPOINT": "localhost:9000",
            "STORAGE_ACCESS_KEY": "minioadmin",
            "STORAGE_SECRET_KEY": "minioadmin",
            "STORAGE_SECURE": "true",
        },
    ],
)
def test_storage(env):
    # TODO: more proper test
    with patch("minio.Minio.make_bucket"), patch("minio.Minio.bucket_exists") as mock_bucket_exists:
        mock_bucket_exists.return_value = True
        storage = Storage.from_environment(env)
        assert storage
        assert storage.client
        assert storage.backtest
        storage.create_bucket("test")
