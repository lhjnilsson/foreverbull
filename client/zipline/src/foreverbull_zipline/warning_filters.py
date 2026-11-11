import warnings


def install_warning_filters() -> None:
    """Ignore the distutils warning from pandas_datareader, and nothing else.

    Zipline's metrics import pandas_datareader, and that package still
    imports distutils. A process-wide ignore would also hide numpy divide
    warnings and pandas deprecations in the returns.
    """
    warnings.filterwarnings("ignore", category=DeprecationWarning, module=r"pandas_datareader\.compat")
