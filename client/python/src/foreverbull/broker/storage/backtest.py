import io

import minio
import pandas as pd


class Backtest:
    def __init__(self, client: minio.Minio) -> None:
        self.client = client

    def list_backtest_results(self) -> list[str | None]:
        return [obj.object_name for obj in self.client.list_objects("backtest-results")]

    def upload_backtest_result(self, backtest: str, result: pd.DataFrame) -> None:
        result.to_pickle("/tmp/result.pkl")
        self.client.fput_object("backtest-results", backtest, "/tmp/result.pkl")

    def download_backtest_results(self, backtest: str) -> pd.DataFrame:
        response = self.client.get_object("backtest-results", f"{backtest}")
        try:
            df = pd.read_pickle(io.BytesIO(response.read()))
            if type(df) is not pd.DataFrame:
                raise ValueError("Data is not a DataFrame")
            return df
        except Exception as e:
            raise ValueError("Data is not a DataFrame") from e
        finally:
            response.close()

    def list_backtest_ingestions(self) -> list[str | None]:
        return [obj.object_name for obj in self.client.list_objects("backtest-ingestions")]

    def upload_backtest_ingestion(self, local_name: str, remote_name: str) -> None:
        self.client.fput_object("backtest-ingestions", remote_name, local_name)

    def download_backtest_ingestion(self, remote_name: str, local_name: str) -> None:
        self.client.fget_object("backtest-ingestions", remote_name, local_name)
