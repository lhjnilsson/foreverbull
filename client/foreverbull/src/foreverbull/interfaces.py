from abc import ABC, abstractmethod
from typing import Any, Dict, Iterator

from foreverbull import entity
from pandas import DataFrame


class Asset(ABC):
    @abstractmethod
    def get_metric[T: (int, float, bool, str, None)](self, key: str) -> T:
        raise NotImplementedError()

    @abstractmethod
    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        raise NotImplementedError()

    @property
    @abstractmethod
    def symbol(self) -> str:
        raise NotImplementedError()

    @property
    @abstractmethod
    def stock_data(self) -> DataFrame:
        raise NotImplementedError()

class Assets(ABC):
    @abstractmethod
    def __iter__(self) -> Iterator[Asset]:
        raise NotImplementedError()

    @abstractmethod
    def get_metrics[T: (int, float, bool, str)](self, key: str) -> dict[str, T]:
        raise NotImplementedError()

    @abstractmethod
    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
        raise NotImplementedError()

    @property
    @abstractmethod
    def symbols(self) -> list[str]:
        raise NotImplementedError()

    @property
    @abstractmethod
    def stock_data(self) -> DataFrame:
        raise NotImplementedError()
