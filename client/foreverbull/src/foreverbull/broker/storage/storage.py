import os

import minio
import urllib3

from urllib3.exceptions import MaxRetryError
from urllib3.exceptions import NewConnectionError

from .backtest import Backtest


class Storage:
    def __init__(self, address, access_key, secret_key, secure=False):
        self.client = minio.Minio(address, access_key=access_key, secret_key=secret_key, secure=secure)
        self.client.bucket_exists("backtest-results")
        self.client.bucket_exists("backtest-ingestions")
        self.backtest: Backtest = Backtest(self.client)

    @classmethod
    def from_environment(cls, env=os.environ):
        address = env.get("STORAGE_ENDPOINT", "localhost:9000")
        http = urllib3.PoolManager()
        try:
            http.request("GET", f"http://{address}", timeout=0.1)
        except (MaxRetryError, NewConnectionError):
            address = "minio:9000"

        return cls(
            address=address,
            access_key=env.get("STORAGE_ACCESS_KEY", "minioadmin"),
            secret_key=env.get("STORAGE_SECRET_KEY", "minioadmin"),
            secure=bool(env.get("STORAGE_SECURE", False)),
        )

    def upload_object(self, bucket: str, remote_name: str, local_name: str) -> None:
        self.client.fput_object(bucket, remote_name, local_name)

    def download_object(self, bucket: str, remote_name: str, local_name: str) -> None:
        self.client.fget_object(bucket, remote_name, local_name)
