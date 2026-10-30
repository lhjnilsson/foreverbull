import http.server
import socket
import threading

import pytest

from foreverbull import exceptions
from foreverbull.broker import storage
from foreverbull.broker.storage import Storage
from foreverbull.broker.storage import StorageConfigurationError


ACCESS = "access-key-value"
SECRET = "secret-key-value"


@pytest.fixture
def http_endpoint():
    """A local HTTP server that answers 403 to everything, like MinIO without credentials."""

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(403)
            self.end_headers()

        def log_message(self, format, *args):
            pass

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    yield f"127.0.0.1:{server.server_address[1]}"
    server.shutdown()
    server.server_close()


@pytest.fixture
def closed_endpoint():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    return f"127.0.0.1:{port}"


def full_env(endpoint: str, **overrides: str) -> dict[str, str]:
    env = {
        "STORAGE_ENDPOINT": endpoint,
        "STORAGE_ACCESS_KEY": ACCESS,
        "STORAGE_SECRET_KEY": SECRET,
    }
    env.update(overrides)
    return env


def test_valid_config_builds_a_client_for_the_given_endpoint(http_endpoint):
    built = Storage.from_environment(full_env(http_endpoint, STORAGE_SECURE="false"))
    assert built.client._base_url.host == http_endpoint
    assert built.client._base_url.is_https is False


def test_unset_secure_is_plain_http(http_endpoint):
    built = Storage.from_environment(full_env(http_endpoint))
    assert built.client._base_url.is_https is False


def test_unreachable_endpoint_fails_instead_of_switching_to_another(closed_endpoint, caplog):
    with pytest.raises(exceptions.ConnectionError) as info:
        Storage.from_environment(full_env(closed_endpoint))
    assert str(info.value) == (
        "Couldn't connect to foreverbull's storage. Run `fbenv status` to check that the environment is running, "
        "then try again."
    )
    assert closed_endpoint not in str(info.value)
    assert info.value.cause is not None
    assert info.value.cause.startswith(f"STORAGE_ENDPOINT={closed_endpoint}: ")
    assert f"STORAGE_ENDPOINT={closed_endpoint}" in caplog.text
    assert "minio:9000" not in caplog.text
    assert SECRET not in caplog.text
    assert ACCESS not in caplog.text


@pytest.mark.parametrize(
    "missing,message",
    [
        (
            ["STORAGE_ACCESS_KEY"],
            (
                "foreverbull's storage isn't set up: STORAGE_ACCESS_KEY is missing. "
                "Set it, or run `fbenv status` if you're using the local environment."
            ),
        ),
        (
            ["STORAGE_SECRET_KEY"],
            (
                "foreverbull's storage isn't set up: STORAGE_SECRET_KEY is missing. "
                "Set it, or run `fbenv status` if you're using the local environment."
            ),
        ),
        (
            ["STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY"],
            (
                "foreverbull's storage isn't set up: STORAGE_ACCESS_KEY and STORAGE_SECRET_KEY are missing. "
                "Set them, or run `fbenv status` if you're using the local environment."
            ),
        ),
        (
            ["STORAGE_ENDPOINT"],
            (
                "foreverbull's storage isn't set up: STORAGE_ENDPOINT is missing. "
                "Set it, or run `fbenv status` if you're using the local environment."
            ),
        ),
        (
            ["STORAGE_ENDPOINT", "STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY"],
            (
                "foreverbull's storage isn't set up: STORAGE_ENDPOINT, STORAGE_ACCESS_KEY and STORAGE_SECRET_KEY "
                "are missing. Set them, or run `fbenv status` if you're using the local environment."
            ),
        ),
    ],
)
def test_missing_variables_are_named_without_a_default(http_endpoint, missing, message):
    env = full_env(http_endpoint)
    for name in missing:
        del env[name]
    with pytest.raises(StorageConfigurationError) as info:
        Storage.from_environment(env)
    assert str(info.value) == message
    assert info.value.variables == missing
    assert "minioadmin" not in str(info.value)
    assert SECRET not in str(info.value)
    assert ACCESS not in str(info.value)


def test_empty_or_blank_key_counts_as_missing(http_endpoint):
    with pytest.raises(StorageConfigurationError) as info:
        Storage.from_environment(full_env(http_endpoint, STORAGE_ACCESS_KEY="", STORAGE_SECRET_KEY="  "))
    assert str(info.value) == (
        "foreverbull's storage isn't set up: STORAGE_ACCESS_KEY and STORAGE_SECRET_KEY are missing. "
        "Set them, or run `fbenv status` if you're using the local environment."
    )


@pytest.mark.parametrize("value", ["yes", "no", "on", "off", "enabled", "2", " ", "false!"])
def test_secure_must_be_true_or_false(http_endpoint, value):
    with pytest.raises(StorageConfigurationError) as info:
        Storage.from_environment(full_env(http_endpoint, STORAGE_SECURE=value))
    assert str(info.value) == f"STORAGE_SECURE must be true or false, but it's set to '{value}'."
    assert info.value.variables == ["STORAGE_SECURE"]
    assert SECRET not in str(info.value)
    assert ACCESS not in str(info.value)


def test_secure_yes_matches_the_copy_exactly(http_endpoint):
    with pytest.raises(StorageConfigurationError) as info:
        Storage.from_environment(full_env(http_endpoint, STORAGE_SECURE="yes"))
    assert str(info.value) == "STORAGE_SECURE must be true or false, but it's set to 'yes'."


def test_missing_key_and_bad_secure_are_named_together(http_endpoint):
    env = full_env(http_endpoint, STORAGE_SECURE="yes")
    del env["STORAGE_ACCESS_KEY"]
    with pytest.raises(StorageConfigurationError) as info:
        Storage.from_environment(env)
    assert str(info.value) == (
        "foreverbull's storage isn't set up:\n"
        "  STORAGE_ACCESS_KEY is missing.\n"
        "  STORAGE_SECURE must be true or false, but it's set to 'yes'.\n"
        "Fix these, or run `fbenv status` if you're using the local environment."
    )
    assert info.value.variables == ["STORAGE_ACCESS_KEY", "STORAGE_SECURE"]
    assert SECRET not in str(info.value)


def test_bad_secure_is_refused_before_the_endpoint_is_dialed(closed_endpoint):
    with pytest.raises(StorageConfigurationError) as info:
        Storage.from_environment(full_env(closed_endpoint, STORAGE_SECURE="yes"))
    assert info.value.variables == ["STORAGE_SECURE"]


@pytest.mark.parametrize(
    "value,expected",
    [
        ("true", True),
        ("TRUE", True),
        ("1", True),
        (" true ", True),
        ("false", False),
        ("False", False),
        ("0", False),
        ("", False),
    ],
)
def test_secure_accepts_true_false_one_zero(value, expected):
    assert storage.parse_secure({"STORAGE_SECURE": value}) is expected


def test_false_string_is_not_truthy():
    assert storage.parse_secure({"STORAGE_SECURE": "false"}) is False


def test_secure_true_probes_over_https(http_endpoint):
    # The local server speaks plain HTTP, so an HTTPS probe cannot complete.
    with pytest.raises(exceptions.ConnectionError) as info:
        Storage.from_environment(full_env(http_endpoint, STORAGE_SECURE="true"))
    assert info.value.cause is not None
    assert info.value.cause.startswith(f"STORAGE_ENDPOINT={http_endpoint}: ")
