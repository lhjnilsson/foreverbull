import importlib
import subprocess
import sys


# Create virtual environment if selected
if eval("{{ cookiecutter.install_dependencies }}"):
    base_repo = "git+https://github.com/lhjnilsson/foreverbull.git@main#egg={pkg}&subdirectory=client/{pkg}"
    foreverbull = base_repo.format(pkg="foreverbull")
    foreverbull_testing = base_repo.format(pkg="foreverbull_testing")
    foreverbull_cli = base_repo.format(pkg="foreverbull_cli")

    for pkg in [foreverbull, foreverbull_testing, foreverbull_cli]:
        code = subprocess.check_call([sys.executable, "-m", "pip", "install", pkg])
        assert code == 0, f"Failed to install {pkg}"

    importlib.invalidate_caches()

if eval("{{ cookiecutter.run_setup }}"):
    try:
        from foreverbull_cli.setup import setup
    except ImportError:
        print("Unable to import foreverbull_cli, did you skip installing dependencies?")
        exit(1)
    setup()
