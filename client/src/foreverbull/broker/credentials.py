import os
import stat

from collections.abc import Mapping
from contextvars import ContextVar
from pathlib import Path

import grpc


TOKEN_ENV = "FOREVERBULL_TOKEN"
SESSION_METADATA = "x-foreverbull-session"
_AUTHORIZATION = "authorization"
_cli_token: ContextVar[str] = ContextVar("foreverbull_cli_token", default="")


class TokenFileError(Exception):
    """A token file the process must not use. The message is a path, never the secret."""


class _CallDetails(grpc.ClientCallDetails):
    def __init__(self, method, timeout, metadata, credentials, wait_for_ready, compression):
        self.method = method
        self.timeout = timeout
        self.metadata = metadata
        self.credentials = credentials
        self.wait_for_ready = wait_for_ready
        self.compression = compression


class _MetadataInterceptor(
    grpc.UnaryUnaryClientInterceptor,
    grpc.UnaryStreamClientInterceptor,
    grpc.StreamUnaryClientInterceptor,
    grpc.StreamStreamClientInterceptor,
):
    def __init__(self, pairs: tuple[tuple[str, str], ...]) -> None:
        self._pairs = pairs

    def _details(self, client_call_details):
        metadata: list[tuple[str, str | bytes]] = []
        if client_call_details.metadata is not None:
            metadata.extend(client_call_details.metadata)
        metadata.extend(self._pairs)
        return _CallDetails(
            client_call_details.method,
            client_call_details.timeout,
            metadata,
            client_call_details.credentials,
            getattr(client_call_details, "wait_for_ready", None),
            getattr(client_call_details, "compression", None),
        )

    def intercept_unary_unary(self, continuation, client_call_details, request):
        return continuation(self._details(client_call_details), request)

    def intercept_unary_stream(self, continuation, client_call_details, request):
        return continuation(self._details(client_call_details), request)

    def intercept_stream_unary(self, continuation, client_call_details, request_iterator):
        return continuation(self._details(client_call_details), request_iterator)

    def intercept_stream_stream(self, continuation, client_call_details, request_iterator):
        return continuation(self._details(client_call_details), request_iterator)


def set_cli_token(value: str | None) -> None:
    """Remember --token for this process. None clears it. The value is not written to disk."""
    if value is None:
        _cli_token.set("")
        return
    _cli_token.set(value.strip())


def token_path(home: Path | None = None) -> Path:
    home = Path.home() if home is None else home
    return home / ".foreverbull" / "token"


def resolve_token(
    *,
    environ: Mapping[str, str] | None = None,
    home: Path | None = None,
) -> str:
    """First match wins: --token, FOREVERBULL_TOKEN, then ~/.foreverbull/token. Empty means no credential."""
    environ = os.environ if environ is None else environ
    home = Path.home() if home is None else home
    cli_token = _cli_token.get().strip()
    if cli_token:
        return cli_token
    env_token = str(environ.get(TOKEN_ENV) or "").strip()
    if env_token:
        return env_token
    return _read_token_file(token_path(home))


def save_token_file(raw: str, *, force: bool, home: Path | None = None) -> Path:
    """Write ~/.foreverbull/token mode 0600. The secret is not an argument and is not logged."""
    token = raw.strip()
    if not token:
        raise TokenFileError("token is empty")
    path = token_path(home)
    if path.exists() and not force:
        raise TokenFileError(f"token file already exists: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    try:
        os.write(fd, token.encode("utf-8"))
    finally:
        os.close(fd)
    os.chmod(path, 0o600)
    return path


def broker_channel(address: str) -> grpc.Channel:
    """Broker channel. No metadata when no token is configured, so authentication-off calls stay open."""
    channel = grpc.insecure_channel(address)
    token = resolve_token()
    if not token:
        return channel
    return grpc.intercept_channel(channel, _MetadataInterceptor(((_AUTHORIZATION, f"Bearer {token}"),)))


def session_channel(address: str, secret: str) -> grpc.Channel:
    """Session channel. The user bearer is not sent. An empty secret adds no metadata."""
    channel = grpc.insecure_channel(address)
    cleaned = secret.strip()
    if not cleaned:
        return channel
    return grpc.intercept_channel(channel, _MetadataInterceptor(((SESSION_METADATA, cleaned),)))


def _read_token_file(path: Path) -> str:
    if not path.is_file():
        return ""
    mode = stat.S_IMODE(path.stat().st_mode)
    if mode & 0o077:
        raise TokenFileError(f"token file mode must be 0600: {path}")
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        raise TokenFileError(f"token file cannot be read: {path}") from None
    return text.strip()
