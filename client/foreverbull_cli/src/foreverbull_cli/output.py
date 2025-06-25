from contextlib import contextmanager

import typer

from rich.console import Console
from rich.console import Group
from rich.live import Live
from rich.progress import BarColumn
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import Task
from rich.progress import TextColumn

from foreverbull.pb.foreverbull import common_pb2


console = Console()


class FBSpinnerColumn(SpinnerColumn):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.finished_text = "[green]✔[/green]"
        self.failed_text = "[red]✘[/red]"

    def render(self, task: Task):
        if not task.started:
            return "-"
        if task.completed < 0:
            return self.failed_text
        return super().render(task)


class LiveProgress(Live):
    def __init__(self):
        self._orchestration_tasks = {}
        spinner = FBSpinnerColumn()
        text = TextColumn("[progress.description]{task.description}")
        self.progress = Progress(spinner, text)

        self._bar_progress = Progress(
            TextColumn("[bold blue]{task.fields[name]}, {task.description}: {task.percentage:.0f}%"),
            BarColumn(),
            TextColumn("({task.completed} of {task.total} steps done)"),
        )
        g = Group(self.progress, self._bar_progress)
        super().__init__(g)

    def __enter__(self) -> "LiveProgress":
        super().__enter__()
        return self

    def update_from_message(self, msg):
        if not self.progress.tasks:
            fields = [
                f for f in msg.DESCRIPTOR.fields if f.message_type.full_name == common_pb2.Step.DESCRIPTOR.full_name
            ]
            for field in fields:
                task_id = self.progress.add_task(
                    " ".join([f.capitalize() for f in field.name.split("_")]), start=False, total=1
                )
                self._orchestration_tasks[field.name] = task_id
        else:
            for desc, step in [
                f for f in msg.ListFields() if f[0].message_type.full_name == common_pb2.Step.DESCRIPTOR.full_name
            ]:
                task_id = self._orchestration_tasks[desc.name]
                match step.state:
                    case common_pb2.State.STATE_RUNNING:
                        self.progress.start_task(task_id)
                    case common_pb2.State.STATE_COMPLETED:
                        if not self.progress.tasks[task_id].started:
                            self.progress.start_task(task_id)
                        self.progress.update(task_id, completed=1)

    @contextmanager
    def bar_progress(self, name, description=""):
        task_id = self._bar_progress.add_task(description, name=name)

        def update(completed, total):
            self._bar_progress.update(task_id, completed=completed, total=total)

        yield update

    def confirm(self, text: str, default: bool = False) -> bool:
        self.stop()
        confirm = typer.confirm(text, default=default)
        self.start()
        return confirm
