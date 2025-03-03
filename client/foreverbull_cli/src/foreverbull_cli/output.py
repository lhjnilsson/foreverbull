from grpc._channel import _InactiveRpcError
from rich.console import Console
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import Task
from rich.progress import TextColumn


console = Console()


class FBSpinnerColumn(SpinnerColumn):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.finished_text = "[green]✔[/green]"
        self.failed_text = "[red]✘[/red]"

    def render(self, task: Task):
        if task.completed == 0:
            return "-"
        if task.completed < 0:
            return self.failed_text
        return super().render(task)


class FBProgress(Progress):
    def __init__(self):
        spinner = FBSpinnerColumn()
        text = TextColumn("[progress.description]{task.description}")
        super().__init__(spinner, text)

    def __enter__(self) -> "Progress":
        return super().__enter__()

    def __exit__(self, exc_type, exc_val, exc_tb):
        if isinstance(exc_val, _InactiveRpcError):
            details = exc_val.details()
        else:
            details = exc_val

        for task in self.tasks:
            if task.total == 2 and task.completed == 1:
                self.update(task.id, description=f"{task.description} FAILED: {details}")
                self.update(task.id, completed=-1)

        super().__exit__(exc_type, exc_val, exc_tb)
