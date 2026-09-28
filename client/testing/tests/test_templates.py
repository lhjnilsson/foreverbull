from pathlib import Path

from cookiecutter.main import cookiecutter

from foreverbull import Algorithm


TEMPLATES = Path(__file__).resolve().parents[2] / "templates"


def _render(name: str, dest: Path) -> Path:
    cookiecutter(
        str(TEMPLATES / name),
        no_input=True,
        extra_context={
            "project_name": name,
            "project_slug": name,
            "install_dependencies": False,
            "run_setup": False,
        },
        output_dir=str(dest),
        overwrite_if_exists=True,
    )
    return dest / name


def test_generated_directory_uses_project_slug(tmp_path):
    for name, slug, package_file in (
        ("simple", "simple_foreverbull", "crossover.py"),
        ("advanced", "advanced_foreverbull", "macd.py"),
    ):
        dest = tmp_path / name
        dest.mkdir()
        cookiecutter(
            str(TEMPLATES / name),
            no_input=True,
            extra_context={
                "install_dependencies": False,
                "run_setup": False,
            },
            output_dir=str(dest),
            overwrite_if_exists=True,
        )
        assert (dest / slug / package_file).is_file()
        assert list(dest.iterdir()) == [dest / slug]


def test_simple_template_renders_algorithm(tmp_path):
    project = _render("simple", tmp_path)
    crossover = project / "crossover.py"
    assert crossover.is_file()
    assert (project / "README.md").is_file()
    assert (project / "backtest.yaml").is_file()
    assert (project / "requirements.txt").is_file()
    algo = Algorithm.from_file_path(str(crossover))
    assert "handle_data" in algo._functions


def test_advanced_template_renders_files(tmp_path):
    project = _render("advanced", tmp_path)
    assert (project / "macd.py").is_file()
    assert (project / "rsi.py").is_file()
    assert (project / "README.md").is_file()
    assert (project / "backtest.yaml").is_file()
    assert (project / "requirements.txt").is_file()
    macd = (project / "macd.py").read_text()
    rsi = (project / "rsi.py").read_text()
    assert "talib" in macd
    assert "talib" in rsi
    assert "order_target_percent" in macd
    assert "order_target_percent" in rsi
    assert "cash_reserve" in macd and "max_positions" in macd
    assert "cash_reserve" in rsi and "max_positions" in rsi
    assert "equal_weight_percent" in macd
    assert "equal_weight_percent" in rsi
