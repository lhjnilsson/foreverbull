import logging
import os

from collections.abc import Mapping

import minio
import urllib3

from foreverbull import exceptions


# Names of the variables, not their values.
ENDPOINT_VAR = "STORAGE_ENDPOINT"
ACCESS_KEY_VAR = "STORAGE_ACCESS_KEY"
SECRET_KEY_VAR = "STORAGE_SECRET_KEY"
SECURE_VAR = "STORAGE_SECURE"

# The reachability probe gives up after this many seconds per phase. It is a
# check before the first real request, not a retry.
PROBE_TIMEOUT_S = 2.0

UNREACHABLE_MESSAGE = (
    "Couldn't connect to foreverbull's storage. Run `fbenv status` to check that the environment is running, "
    "then try again."
)

log = logging.getLogger(__name__)


class StorageConfigurationError(exceptions.ConfigurationError):
    """A storage variable is missing or invalid. `variables` names them; no value of a key is kept."""

    def __init__(self, message: str, variables: list[str]):
        super().__init__(message)
        self.variables = variables


def _join_names(names: list[str]) -> str:
    if len(names) == 1:
        return names[0]
    return ", ".join(names[:-1]) + " and " + names[-1]


def missing_message(names: list[str]) -> str:
    verb, pronoun = ("is", "it") if len(names) == 1 else ("are", "them")
    return (
        f"foreverbull's storage isn't set up: {_join_names(names)} {verb} missing. "
        f"Set {pronoun}, or run `fbenv status` if you're using the local environment."
    )


def bad_secure_message(raw: str) -> str:
    return f"{SECURE_VAR} must be true or false, but it's set to '{raw}'."


def combined_message(missing: list[str], raw_secure: str) -> str:
    """Name every missing variable and the bad STORAGE_SECURE together. Only the STORAGE_SECURE value is echoed."""
    verb = "is" if len(missing) == 1 else "are"
    return (
        "foreverbull's storage isn't set up:\n"
        f"  {_join_names(missing)} {verb} missing.\n"
        f"  {bad_secure_message(raw_secure)}\n"
        "Fix these, or run `fbenv status` if you're using the local environment."
    )


def _read_secure(env: Mapping[str, str]) -> tuple[bool, str | None]:
    """Return (secure, None), or (False, raw) when STORAGE_SECURE holds something other than a boolean."""
    raw = env.get(SECURE_VAR)
    if raw is None or raw == "":
        return False, None
    value = raw.strip().lower()
    if value in ("true", "1"):
        return True, None
    if value in ("false", "0"):
        return False, None
    return False, raw


def parse_secure(env: Mapping[str, str]) -> bool:
    """Read STORAGE_SECURE. Unset is False: the chart's in-cluster MinIO serves plain HTTP.

    true, false, 1, and 0 are accepted in any case, with surrounding spaces ignored. Anything else
    is refused rather than read as truthy.
    """
    secure, bad = _read_secure(env)
    if bad is not None:
        raise StorageConfigurationError(bad_secure_message(bad), [SECURE_VAR])
    return secure


class Storage:
    def __init__(self, address, access_key, secret_key, secure=False):
        self.client = minio.Minio(address, access_key=access_key, secret_key=secret_key, secure=secure)

    @classmethod
    def from_environment(cls, env: Mapping[str, str] = os.environ):
        """Build a client from STORAGE_* variables, or raise.

        No variable has a default endpoint or key, and an endpoint that does not answer is not
        replaced by another one.
        """
        # The endpoint is read on its own so it never shares a container with the keys it is logged next to.
        address = (env.get(ENDPOINT_VAR) or "").strip()
        access_key = (env.get(ACCESS_KEY_VAR) or "").strip()
        secret_key = (env.get(SECRET_KEY_VAR) or "").strip()
        missing = []
        if not address:
            missing.append(ENDPOINT_VAR)
        if not access_key:
            missing.append(ACCESS_KEY_VAR)
        if not secret_key:
            missing.append(SECRET_KEY_VAR)
        secure, bad_secure = _read_secure(env)
        # Every problem is named in one error before anything is dialed. The message and `variables`
        # name the variables; nothing is logged next to the keys.
        if missing and bad_secure is not None:
            raise StorageConfigurationError(combined_message(missing, bad_secure), [*missing, SECURE_VAR])
        if missing:
            raise StorageConfigurationError(missing_message(missing), missing)
        if bad_secure is not None:
            raise StorageConfigurationError(bad_secure_message(bad_secure), [SECURE_VAR])
        probe(address, secure)
        return cls(address=address, access_key=access_key, secret_key=secret_key, secure=secure)

    def upload_object(self, bucket: str, remote_name: str, local_name: str) -> None:
        self.client.fput_object(bucket, remote_name, local_name)

    def download_object(self, bucket: str, remote_name: str, local_name: str) -> None:
        self.client.fget_object(bucket, remote_name, local_name)


def probe(address: str, secure: bool) -> None:
    """Raise ConnectionError when nothing answers HTTP at the endpoint. Any response counts as reachable."""
    scheme = "https" if secure else "http"
    http = urllib3.PoolManager(retries=False, timeout=urllib3.Timeout(connect=PROBE_TIMEOUT_S, read=PROBE_TIMEOUT_S))
    try:
        http.request("GET", f"{scheme}://{address}")
    except urllib3.exceptions.HTTPError as err:
        cause = f"{ENDPOINT_VAR}={address}: {type(err).__name__}"
        log.error("couldn't connect to storage at %s=%s (%s): %s", ENDPOINT_VAR, address, scheme, type(err).__name__)
        raise exceptions.ConnectionError(UNREACHABLE_MESSAGE, cause=cause) from err
    finally:
        http.clear()
