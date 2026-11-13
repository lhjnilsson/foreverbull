from multiprocessing import get_start_method
from unittest.mock import MagicMock

import foreverbull_cli
import pytest

from foreverbull_cli import StartMethodError
from foreverbull_cli import configure_start_method


def test_importing_the_package_leaves_spawn_set():
    assert get_start_method(allow_none=True) == "spawn"


def test_start_method_none_sets_spawn(monkeypatch):
    set_method = MagicMock()
    monkeypatch.setattr(foreverbull_cli, "get_start_method", lambda allow_none=True: None)
    monkeypatch.setattr(foreverbull_cli, "set_start_method", set_method)
    configure_start_method()
    set_method.assert_called_once_with("spawn")


def test_start_method_spawn_is_left_alone(monkeypatch):
    set_method = MagicMock()
    monkeypatch.setattr(foreverbull_cli, "get_start_method", lambda allow_none=True: "spawn")
    monkeypatch.setattr(foreverbull_cli, "set_start_method", set_method)
    configure_start_method()
    set_method.assert_not_called()


def test_start_method_fork_fails(monkeypatch):
    set_method = MagicMock()
    monkeypatch.setattr(foreverbull_cli, "get_start_method", lambda allow_none=True: "fork")
    monkeypatch.setattr(foreverbull_cli, "set_start_method", set_method)
    with pytest.raises(StartMethodError) as raised:
        configure_start_method()
    assert str(raised.value) == (
        "Python's multiprocessing start method is already set to 'fork', but foreverbull needs 'spawn'. "
        "Remove the code that sets it to 'fork' before foreverbull starts."
    )
    set_method.assert_not_called()
