"""Inspect or age a backtest bundle inside the Zipline pod.

The Kind demo pipes this file to `python -` in the Zipline pod, which has
Zipline, the foreverbull client, and the Minio credentials. It is not a
pytest module.

    python - stale <backtest>   rewrite the newest ingest the way the
                                pre-alignment ingester left it and store
                                it as the backtest's ingestion
    python - state <backtest>   print the ingest count, the newest
                                ingest's version, and whether its asset
                                dates are misaligned, as JSON
"""

import json
import shutil
import sqlite3
import sys
import tarfile

from contextlib import closing
from pathlib import Path


# The image copies foreverbull_zipline to /app and runs it from there; the
# import must not depend on the working directory kubectl exec starts in.
sys.path.insert(0, "/app")

from foreverbull_zipline.data_bundles.bundle_alignment import BUNDLE_VERSION_MARKER
from foreverbull_zipline.data_bundles.bundle_alignment import bundle_dates_misaligned
from foreverbull_zipline.data_bundles.bundle_alignment import bundle_version
from foreverbull_zipline.data_bundles.bundle_alignment import latest_ingest_dir
from zipline.utils.paths import data_path

from foreverbull.broker.storage import Storage


# internal/storage: storage.IngestionsBucket. A run downloads the newest
# object in this bucket, named after the backtest.
INGESTIONS_BUCKET = "ingestions"
# The old ingester kept Yahoo's 09:30 ET (14:30 UTC) bar stamps as the
# asset dates instead of the midnight session label.
SESSION_OPEN_NS = (14 * 60 + 30) * 60 * 1_000_000_000
STALE_DATE_COLUMNS = {
    "equities": ("start_date", "end_date", "auto_close_date"),
    "equity_symbol_mappings": ("start_date", "end_date"),
}


def ingest_dirs(backtest: str) -> list[Path]:
    bundle_dir = Path(data_path([backtest]))
    if not bundle_dir.is_dir():
        return []
    return sorted(entry for entry in bundle_dir.iterdir() if entry.is_dir() and not entry.name.startswith("."))


def make_stale(backtest: str) -> None:
    newest = latest_ingest_dir(backtest)
    if newest is None:
        sys.exit(f"backtest {backtest!r} has no ingest in this pod")
    # Only the aged ingest may stay, or the rebuild check reads a newer one.
    for ingest in ingest_dirs(backtest):
        if ingest != newest:
            shutil.rmtree(ingest)
    (newest / BUNDLE_VERSION_MARKER).unlink(missing_ok=True)
    db_paths = sorted(newest.glob("assets-*.sqlite"))
    if not db_paths:
        sys.exit(f"backtest {backtest!r} has no asset database in {newest}")
    with closing(sqlite3.connect(db_paths[0])) as connection:
        for table, columns in STALE_DATE_COLUMNS.items():
            for column in columns:
                # Dates before 1970 are Zipline's no-date sentinel, not a session.
                connection.execute(
                    f"UPDATE {table} SET {column} = {column} + ? WHERE {column} > 0",
                    (SESSION_OPEN_NS,),
                )
        connection.commit()
    if not bundle_dates_misaligned(backtest):
        sys.exit(f"backtest {backtest!r} still reads as session aligned after aging")
    archive = f"/tmp/{backtest}-stale.tar.gz"
    with tarfile.open(archive, "w:gz") as tar:
        tar.add(data_path([backtest]), arcname=backtest)
    Storage.from_environment().upload_object(INGESTIONS_BUCKET, backtest, archive)


def print_state(backtest: str) -> None:
    state = {
        "ingests": len(ingest_dirs(backtest)),
        "version": bundle_version(backtest),
        "misaligned": bundle_dates_misaligned(backtest),
    }
    print(json.dumps(state))


def main(argv: list[str]) -> None:
    if len(argv) != 3:
        sys.exit("usage: python - stale|state <backtest>")
    action, backtest = argv[1], argv[2]
    if action == "stale":
        make_stale(backtest)
    elif action == "state":
        print_state(backtest)
    else:
        sys.exit(f"unknown action {action!r}: use stale or state")


main(sys.argv)
