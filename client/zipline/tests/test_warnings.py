"""The data service ignores one known library warning, not every warning."""

import warnings

import numpy as np

from foreverbull_zipline.data_bundles.foreverbull import install_warning_filters


def _ignored_deprecations() -> list[str]:
    texts: list[str] = []
    for action, _message, category, module, _lineno in warnings.filters:
        if action != "ignore" or category is not DeprecationWarning or module is None:
            continue
        pattern = getattr(module, "pattern", module)
        if isinstance(pattern, str):
            texts.append(pattern)
    return texts


def test_the_data_service_does_not_ignore_every_warning():
    install_warning_filters()
    assert ("ignore", None, Warning, None, 0) not in warnings.filters
    assert any("pandas_datareader" in text for text in _ignored_deprecations())


def test_a_numpy_divide_warning_is_not_silenced():
    install_warning_filters()
    with warnings.catch_warnings(record=True) as caught, np.errstate(divide="warn", invalid="warn"):
        np.array([1.0]) / 0.0
    assert any(issubclass(item.category, RuntimeWarning) for item in caught)


def test_a_pandas_deprecation_is_not_silenced():
    install_warning_filters()
    with warnings.catch_warnings(record=True) as caught:
        warnings.warn("fillna with method is deprecated", FutureWarning, stacklevel=1)
    assert any(issubclass(item.category, FutureWarning) for item in caught)
