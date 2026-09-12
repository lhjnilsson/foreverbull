import importlib
import shutil
import subprocess
import sys

from pathlib import Path


REPO = "git+https://github.com/lhjnilsson/foreverbull.git@main"
PACKAGES = [
    f"foreverbull @ {REPO}#subdirectory=client",
    f"foreverbull-testing @ {REPO}#subdirectory=client/testing",
    f"foreverbull-cli @ {REPO}#subdirectory=client/cli",
]


def _flag(value: str) -> bool:
    return value.strip().lower() in {"1", "true", "yes", "y"}


Path("requirements.txt").write_text("\n".join(PACKAGES) + "\n")

if _flag("{{ cookiecutter.install_dependencies }}"):
    if shutil.which("uv"):
        subprocess.check_call(["uv", "pip", "install", *PACKAGES])
    else:
        subprocess.check_call([sys.executable, "-m", "pip", "install", *PACKAGES])
    importlib.invalidate_caches()

if _flag("{{ cookiecutter.run_setup }}"):
    print("Next steps:")
    print("  fbenv create")
    print("  fbull backtest create --file backtest.yaml")
    print("  fbull backtest run demo {{ cookiecutter.project_slug }}/crossover.py --yes")
    print("  python -m pytest")
