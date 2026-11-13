from multiprocessing import get_start_method
from multiprocessing import set_start_method


REQUIRED_START_METHOD = "spawn"


class StartMethodError(RuntimeError):
    """The process start method is already set to something other than spawn."""


def configure_start_method() -> None:
    """Use spawn. Spawn already set is left alone.

    No method set means this process chooses spawn. A different method already
    set is a conflict: keeping it would run workers with that method and not
    say so.
    """
    method = get_start_method(allow_none=True)
    if method is None:
        set_start_method(REQUIRED_START_METHOD)
        return
    if method == REQUIRED_START_METHOD:
        return
    raise StartMethodError(
        f"Python's multiprocessing start method is already set to {method!r}, "
        f"but foreverbull needs {REQUIRED_START_METHOD!r}. "
        f"Remove the code that sets it to {method!r} before foreverbull starts."
    )


try:
    configure_start_method()
except StartMethodError as exc:
    # Import of the CLI is the moment the method is chosen. SystemExit prints
    # this line and stops, without a traceback from the failed import.
    raise SystemExit(str(exc)) from None
