from .execution import Execution

try:
    from ._version import version
except ImportError:
    version = "git"

__all__ = ["Execution"]
