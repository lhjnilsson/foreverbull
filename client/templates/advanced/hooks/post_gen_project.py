if eval("{{ cookiecutter.install_dependencies }}"):
    print("Installing dependencies")
else:
    print("no install")