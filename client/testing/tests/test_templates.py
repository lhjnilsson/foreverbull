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


def test_simple_template_renders_algorithm(tmp_path):
    project = _render("simple", tmp_path)
    crossover = project / "simple" / "crossover.py"
    assert crossover.is_file()
    assert (project / "README.md").is_file()
    assert (project / "backtest.yaml").is_file()
    assert (project / "requirements.txt").is_file()
    algo = Algorithm.from_file_path(str(crossover))
    assert "handle_data" in algo._functions


def test_advanced_template_renders_files(tmp_path):
    project = _render("advanced", tmp_path)
    assert (project / "advanced" / "macd.py").is_file()
    assert (project / "advanced" / "rsi.py").is_file()
    assert (project / "README.md").is_file()
    assert (project / "backtest.yaml").is_file()
    assert (project / "requirements.txt").is_file()
    macd = (project / "advanced" / "macd.py").read_text()
    rsi = (project / "advanced" / "rsi.py").read_text()
    assert "talib" in macd
    assert "talib" in rsi
    assert "order_target_percent" in macd
    assert "order_target_percent" in rsi
    assert "cash_reserve" in macd and "max_positions" in macd
    assert "cash_reserve" in rsi and "max_positions" in rsi
    assert "equal_weight_percent" in macd
    assert "equal_weight_percent" in rsi
