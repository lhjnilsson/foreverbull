# Create virtual environment if selected
if "{{ cookiecutter.install_dependencies }}":
    print("Creating virtual environment...")

if "{{ cookiecutter.run_setup }}":
    try:
        from foreverbull_cli.setup import setup
    except ImportError:
        print("Unable to import foreverbull_cli, did you skip installing dependencies?")
        exit(1)
    setup()
