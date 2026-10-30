import os

from contextlib import contextmanager
from typing import Self
from urllib.parse import urlencode
from urllib.parse import urlparse
from urllib.parse import urlunparse

import typer

from protobuf import DescFieldValueMessage
from rich.console import Console
from rich.console import Group
from rich.live import Live
from rich.progress import BarColumn
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import Task
from rich.progress import TextColumn

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.pb_utils import from_proto_date_to_pydate


console = Console()
GRAFANA_URL_ENV = "GRAFANA_URL"
GRAFANA_URL_DEFAULT = "http://localhost:3000"
GRAFANA_DASHBOARD_UID = "ee9ep4jxtwkcgf"
GRAFANA_DASHBOARD_URL = f"{GRAFANA_URL_DEFAULT}/d/{GRAFANA_DASHBOARD_UID}"


def format_proto_date(value: common_pb.Date | None) -> str:
    if value is None:
        return ""
    try:
        return from_proto_date_to_pydate(value).isoformat()
    except ValueError:
        return ""


def format_date_field(message, field: str) -> str:
    if not message.has_field(field):
        return ""
    return format_proto_date(getattr(message, field))


def format_optional_float(message, field: str) -> str:
    if not message.has_field(field):
        return "-"
    return f"{getattr(message, field):.5f}"


def grafana_execution_url(grafana_url: str, execution_id: str) -> str:
    base = (grafana_url or "").strip() or GRAFANA_URL_DEFAULT
    parsed = urlparse(base)
    if not parsed.scheme or not parsed.netloc:
        parsed = urlparse(GRAFANA_URL_DEFAULT)
    path = parsed.path.rstrip("/") + f"/d/{GRAFANA_DASHBOARD_UID}"
    query = urlencode({"var-execution": execution_id})
    return urlunparse((parsed.scheme, parsed.netloc, path, "", query, ""))


def execution_link(execution_id: str) -> str:
    return grafana_execution_url(os.environ.get(GRAFANA_URL_ENV, GRAFANA_URL_DEFAULT), execution_id)


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
        # labels are the step rows' own names ("Downloading"). A running step's
        # progress text replaces its row in place and the label comes back
        # when the step ends.
        self._labels: dict[str, str] = {}
        # failure is the first failed step's message, so a caller can show it
        # as the one error line instead of the progress rows.
        self.failure: str | None = None
        # notices are lines from the notices field of completed steps, such as
        # prices the ingest filled; the caller prints them after the rows.
        # A step's message is failure text only, so it is never read here.
        self.notices: list[str] = []
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

    def __enter__(self) -> Self:
        super().__enter__()
        return self

    def update_from_message(self, msg):
        step_type = common_pb.Step.desc().type_name

        def is_step(field) -> bool:
            return isinstance(field.value, DescFieldValueMessage) and field.value.message.type_name == step_type

        if not self.progress.tasks:
            for field in msg.desc().fields:
                if not is_step(field):
                    continue
                label = " ".join([part.capitalize() for part in field.name.split("_")])
                task_id = self.progress.add_task(label, start=False, total=1)
                self._orchestration_tasks[field.name] = task_id
                self._labels[field.name] = label
        else:
            for field in msg:
                if not is_step(field):
                    continue
                step = msg[field]
                task_id = self._orchestration_tasks[field.name]
                label = self._labels[field.name]
                match step.state:
                    case common_pb.State.RUNNING:
                        if not self.progress.tasks[task_id].started:
                            self.progress.start_task(task_id)
                        # The broker sends progress such as "Downloading prices:
                        # 1,240 of 10,000 symbols" as the running step's message.
                        self.progress.update(task_id, description=step.message or label)
                    case common_pb.State.COMPLETED:
                        if not self.progress.tasks[task_id].started:
                            self.progress.start_task(task_id)
                        if self.progress.tasks[task_id].completed < 1:
                            self.notices.extend(
                                line for notice in step.notices for line in notice.splitlines() if line.strip()
                            )
                        self.progress.update(task_id, completed=1, description=label)
                    case common_pb.State.FAILED:
                        if self.failure is None:
                            self.failure = step.message
                        description = f"{label}: {step.message}"
                        self.progress.update(task_id, completed=-1, description=description)

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
