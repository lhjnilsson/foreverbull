import os

import minio
import requests

from .backtest import Backtest


class Storage:
    def __init__(
        self,
        address,
        access_key,
        secret_key,
        secure=False,
    ):
        self.client = minio.Minio(
            address,
            access_key=access_key,
            secret_key=secret_key,
            secure=secure,
        )
        self.client.bucket_exists("backtest-results")
        self.client.bucket_exists("backtest-ingestions")
        self.backtest: Backtest = Backtest(self.client)

    @classmethod
    def from_environment(
        cls,
        env=os.environ,
    ):
        address = env.get(
            "STORAGE_ENDPOINT",
            "localhost:9000",
        )
        try:
            requests.get(
                f"http://{address}",
                timeout=0.3,
            )
        except requests.exceptions.ConnectionError:
            address = "minio:9000"

        return cls(
            address=address,
            access_key=env.get(
                "STORAGE_ACCESS_KEY",
                "minioadmin",
            ),
            secret_key=env.get(
                "STORAGE_SECRET_KEY",
                "minioadmin",
            ),
            secure=bool(
                env.get(
                    "STORAGE_SECURE",
                    False,
                )
            ),
        )

    def create_bucket(
        self,
        bucket_name,
    ):
        self.client.make_bucket(bucket_name)
