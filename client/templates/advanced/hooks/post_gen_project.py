# Create virtual environment if selected

print("okok {{ cookiecutter.install_dependencies }}")

if eval("{{ cookiecutter.install_dependencies }}"):
    print("Installing dependencies")
else:
    print("no install")