# Warn if Docker is missing. Generation still proceeds so unit tests can render
# templates without a daemon; fbenv and Testcontainers need Docker later.
import json
import subprocess


try:
    result = subprocess.run(
        ["docker", "version", "-f", "json"],
        capture_output=True,
        text=True,
        check=False,
    )
    json.loads(result.stdout)
except (OSError, json.JSONDecodeError):
    print("Warning: Docker is not running or is not installed.")
    print("Start Docker before using fbenv or pytest fixtures that need Postgres.")
