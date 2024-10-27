from rich.console import Console
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import Task
from rich.progress import TextColumn


console = Console()


class FBSpinnerColumn(SpinnerColumn):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs, finished_text="[green]✔[/green]")

    def render(self, task: Task):
        if task.completed == 0:
            return "-"
        return super().render(task)


class FBProgress(Progress):
    def __init__(self):
        spinner = FBSpinnerColumn()
        text = TextColumn("[progress.description]{task.description}")
        super().__init__(spinner, text)
