from typing import Annotated
from typing import Self

import typer

from grpc._channel import _InactiveRpcError
from rich.progress import Progress
from rich.progress import TextColumn
from rich.table import Table

from foreverbull_cli.env.environment import ClusterManager
from foreverbull_cli.env.environment import Config
from foreverbull_cli.env.environment import Environment
from foreverbull_cli.env.environment import ImagePullError
from foreverbull_cli.env.environment import MissingToolError
from foreverbull_cli.env.preflight import CheckState
from foreverbull_cli.env.preflight import PreflightError
from foreverbull_cli.output import FBSpinnerColumn
from foreverbull_cli.output import console


class FBProgress(Progress):
    def __init__(self):
        spinner = FBSpinnerColumn()
        text = TextColumn("[progress.description]{task.description}")
        super().__init__(spinner, text)

    def __enter__(self) -> Self:
        super().__enter__()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if isinstance(exc_val, _InactiveRpcError):
            details = exc_val.details()
        else:
            details = exc_val

        for task in self.tasks:
            if task.total == 2 and task.completed == 1:
                if isinstance(exc_val, ImagePullError):
                    # The ✖ line printed after the progress says what failed.
                    self.update(task.id, description=f"{task.description} FAILED")
                else:
                    self.update(task.id, description=f"{task.description} FAILED: {details}")
                self.update(task.id, completed=-1)

        super().__exit__(exc_type, exc_val, exc_tb)


cli = typer.Typer()

ClusterFlag = Annotated[str | None, typer.Option("--cluster", help="local (Kind) or remote kube context")]
KubeconfigFlag = Annotated[str | None, typer.Option("--kubeconfig", help="Kubeconfig path (remote)")]
ContextFlag = Annotated[str | None, typer.Option("--context", help="Kube context (remote)")]
NamespaceFlag = Annotated[str | None, typer.Option("--namespace", help="Helm namespace")]
BrokerAddressFlag = Annotated[str | None, typer.Option("--broker-address", help="Broker host:port")]
VerboseFlag = Annotated[bool, typer.Option("--verbose", "-v", help="Show Docker's own error when an image pull fails")]


def _manager(
    ctx: typer.Context,
    *,
    cluster: str | None = None,
    kubeconfig: str | None = None,
    context: str | None = None,
    namespace: str | None = None,
    broker_address: str | None = None,
) -> ClusterManager:
    cm: ClusterManager = ctx.obj
    cm.config.apply_cli_overrides(
        cluster=cluster,
        kubeconfig=kubeconfig,
        context=context,
        namespace=namespace,
        broker_address=broker_address,
    )
    return cm


def print_status(cm: ClusterManager):
    cfg = cm.config
    broker = cfg.broker_address or "unset"
    title = (
        f"Environment Status ({cfg.cluster}, ns={cfg.namespace}, "
        f"context={cfg.kube_context or 'default'}, broker={broker})"
    )
    table = Table(title=title)
    table.add_column("Service")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Version")
    table.add_column("Container ID")
    try:
        rows = cm.status_rows()
    except MissingToolError as exc:
        console.print(f"Workload status needs kubectl. {exc}")
        return
    for row in rows:
        table.add_row(row.service, row.name, row.status, row.image, row.container_id)
    console.print(table)
    diagnosis = cm.broker_diagnosis()
    if diagnosis is not None:
        console.print(diagnosis.advice)
        console.print(diagnosis.detail, markup=False, highlight=False)


CHECK_STYLES = {
    CheckState.PASS: "[green]pass[/green]",
    CheckState.FAIL: "[red]fail[/red]",
    CheckState.SKIP: "[dim]skip[/dim]",
}


def print_host_checks(cm: ClusterManager):
    table = Table(title="Host checks")
    table.add_column("Check")
    table.add_column("Result")
    table.add_column("Detail")
    for check in cm.host_checks():
        detail = check.detail
        if check.hint:
            detail = f"{detail}\n{check.hint}"
        table.add_row(check.name, CHECK_STYLES[check.state], detail)
    console.print(table)


def run_preflight(cm: ClusterManager):
    """Stop with the failed check and its fix hint before anything changes."""
    try:
        cm.preflight()
    except PreflightError as exc:
        console.print(str(exc), markup=False, highlight=False)
        raise typer.Exit(1) from exc


def fail_image_pull(exc: ImagePullError, command: str, *, verbose: bool, nothing_changed: bool):
    """Stop with one ✖ line that names the image and the cause. Docker's output only with --verbose."""
    lines = [f"✖ {exc}", exc.hint(command)]
    if nothing_changed:
        lines.append("Nothing was changed.")
    if verbose and exc.detail:
        lines.append(f"Docker said: {exc.detail}")
    elif exc.detail:
        lines.append(f"Run `{command} --verbose` to see Docker's error.")
    console.print("\n".join(lines), markup=False, highlight=False, soft_wrap=True)
    raise typer.Exit(1) from exc


def print_cluster(config: Config, *, path_note: str | None = None):
    if path_note:
        console.print(path_note)
    console.print(f"cluster: {config.cluster}")
    console.print(f"namespace: {config.namespace}")
    console.print(f"context: {config.kube_context or '(default)'}")
    console.print(f"kubeconfig: {config.kubeconfig or '(default)'}")
    console.print(f"broker: {config.broker_address or '(unset)'}")


@cli.command(help="Show host checks and the selected cluster")
def status(
    ctx: typer.Context,
    cluster: ClusterFlag = None,
    kubeconfig: KubeconfigFlag = None,
    context: ContextFlag = None,
    namespace: NamespaceFlag = None,
    broker_address: BrokerAddressFlag = None,
):
    cm = _manager(
        ctx,
        cluster=cluster,
        kubeconfig=kubeconfig,
        context=context,
        namespace=namespace,
        broker_address=broker_address,
    )
    print_host_checks(cm)
    print_status(cm)


@cli.command(help="Install the Kind cluster and Helm release. Workloads stay stopped.")
def create(
    ctx: typer.Context,
    start: Annotated[bool, typer.Option("--start", help="Start workloads after install")] = False,
    cluster: ClusterFlag = None,
    kubeconfig: KubeconfigFlag = None,
    context: ContextFlag = None,
    namespace: NamespaceFlag = None,
    broker_address: BrokerAddressFlag = None,
    verbose: VerboseFlag = False,
):
    cm = _manager(
        ctx,
        cluster=cluster,
        kubeconfig=kubeconfig,
        context=context,
        namespace=namespace,
        broker_address=broker_address,
    )
    run_preflight(cm)

    changing = False
    try:
        with FBProgress() as progress:
            verify_images = progress.add_task("Verifying images", total=2)
            create_task = progress.add_task("Creating environment", total=2)
            if start:
                start_task = progress.add_task("Starting environment", total=2)
            else:
                start_task = None

            progress.update(verify_images, completed=1)
            cm.verify_images()
            progress.update(verify_images, completed=2)

            changing = True
            progress.update(create_task, completed=1)
            cm.create()

            progress.update(create_task, completed=2)
            if start_task:
                progress.update(start_task, completed=1)
                cm.start()
                progress.update(start_task, completed=2)
    except ImagePullError as exc:
        fail_image_pull(exc, "fbenv create", verbose=verbose, nothing_changed=not changing)

    print_status(cm)
    if not start:
        console.print("Workloads are stopped. Start them with `fbenv start`.")


@cli.command(help="Scale workloads to 1 and wait until they are ready.")
def start(
    ctx: typer.Context,
    cluster: ClusterFlag = None,
    kubeconfig: KubeconfigFlag = None,
    context: ContextFlag = None,
    namespace: NamespaceFlag = None,
    broker_address: BrokerAddressFlag = None,
):
    cm = _manager(
        ctx,
        cluster=cluster,
        kubeconfig=kubeconfig,
        context=context,
        namespace=namespace,
        broker_address=broker_address,
    )
    run_preflight(cm)

    try:
        with FBProgress() as progress:
            start_task = progress.add_task("Starting environment", total=2)
            progress.update(start_task, completed=1)
            cm.start()
            progress.update(start_task, completed=2)
    except TimeoutError as exc:
        console.print(str(exc))
        raise typer.Exit(1) from exc

    print_status(cm)


@cli.command(help="Scale workloads to 0.")
def stop(
    ctx: typer.Context,
    remove: bool = typer.Option(False, "--remove", help="Uninstall the Helm release"),
    purge: bool = typer.Option(
        False,
        "--purge",
        help="With --remove, delete the Kind cluster (local) and .foreverbull data",
    ),
    cluster: ClusterFlag = None,
    kubeconfig: KubeconfigFlag = None,
    context: ContextFlag = None,
    namespace: NamespaceFlag = None,
    broker_address: BrokerAddressFlag = None,
):
    cm = _manager(
        ctx,
        cluster=cluster,
        kubeconfig=kubeconfig,
        context=context,
        namespace=namespace,
        broker_address=broker_address,
    )

    with FBProgress() as progress:
        stop_task = progress.add_task("Stopping environment", total=2)
        if remove:
            remove_task = progress.add_task("Removing environment", total=2)
        else:
            remove_task = None

        progress.update(stop_task, completed=1)
        cm.stop()
        progress.update(stop_task, completed=2)

        if remove_task:
            progress.update(remove_task, completed=1)
            cm.remove(purge)
            progress.update(remove_task, completed=2)
    print_status(cm)


@cli.command(help="update environment images")
def update(
    ctx: typer.Context,
    cluster: ClusterFlag = None,
    kubeconfig: KubeconfigFlag = None,
    context: ContextFlag = None,
    namespace: NamespaceFlag = None,
    broker_address: BrokerAddressFlag = None,
    verbose: VerboseFlag = False,
):
    cm = _manager(
        ctx,
        cluster=cluster,
        kubeconfig=kubeconfig,
        context=context,
        namespace=namespace,
        broker_address=broker_address,
    )

    try:
        with FBProgress() as progress:
            update_task = progress.add_task("Updating environment", total=2)
            progress.update(update_task, completed=1)
            cm.update()
            progress.update(update_task, completed=2)
    except ImagePullError as exc:
        # update pulls every missing image before kind load and helm upgrade, so nothing changed yet.
        fail_image_pull(exc, "fbenv update", verbose=verbose, nothing_changed=True)

    print_status(cm)


@cli.command(help="Select cluster: local (Kind) or remote. Writes .foreverbull/config.yaml")
def use(
    ctx: typer.Context,
    cluster: Annotated[str | None, typer.Argument(help="local or remote")] = None,
    kubeconfig: KubeconfigFlag = None,
    context: ContextFlag = None,
    namespace: NamespaceFlag = None,
    broker_address: BrokerAddressFlag = None,
):
    cm: ClusterManager = ctx.obj
    config: Config = cm.config
    if cluster is None:
        print_cluster(config)
        return
    try:
        path = config.persist_selection(
            cluster,
            kubeconfig=kubeconfig,
            context=context,
            namespace=namespace,
            broker_address=broker_address,
        )
    except ValueError as exc:
        raise typer.BadParameter(str(exc)) from exc
    print_cluster(config, path_note=f"Wrote {path}")


@cli.callback()
def initialize(ctx: typer.Context):
    config = Config()
    environment = Environment()
    ctx.obj = ClusterManager(environment, config)
