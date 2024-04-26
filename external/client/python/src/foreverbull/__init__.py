import importlib
import logging
import os
from functools import wraps
from inspect import getabsfile, signature

from foreverbull import entity
from foreverbull._version import version  # noqa
from foreverbull.data import Asset, Assets
from foreverbull.entity.finance import Portfolio

log_level = os.environ.get("LOGLEVEL", "WARNING").upper()
logging.basicConfig(level=log_level)


def algo(f):
    @wraps(f)
    def wrapper(f):
        def eval_param(type: any) -> str:
            if type == int:
                return "int"
            elif type == float:
                return "float"
            elif type == bool:
                return "bool"
            elif type == str:
                return "str"
            else:
                raise Exception("Unknown parameter type: {}".format(type))

        parameters = []
        file_path = getabsfile(f)
        parallel = False
        for key, value in signature(f).parameters.items():
            if value.annotation == Assets:
                parallel = False
                continue
            elif value.annotation == Asset:
                parallel = True
                continue
            elif value.annotation == Portfolio:
                continue
            default = None if value.default == value.empty else str(value.default)
            parameter = entity.service.Parameter(key=key, default=default, type=eval_param(value.annotation))
            parameters.append(parameter)
        f._algo = {"parameters": parameters, "file_path": file_path, "parallel": parallel, "func": f}
        return f

    return wrapper(f)


def import_file(file_path: str) -> dict:
    spec = importlib.util.spec_from_file_location("", file_path)
    source = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(source)
    for part in dir(source):
        if getattr(source, part) is None:
            continue
        if hasattr(getattr(source, part), "_algo"):
            return getattr(source, part)._algo
    raise Exception("No algo found in {}".format(file_path))


from foreverbull.foreverbull import Foreverbull  # noqa: E402

__all__ = [Foreverbull, Asset, Portfolio, algo]
