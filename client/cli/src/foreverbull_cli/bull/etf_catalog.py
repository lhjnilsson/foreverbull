"""Known iShares ETFs.

The broker embeds the same file:
pkg/backtest/internal/stream/dependency/etf_catalog.json.
"""

import json

from dataclasses import dataclass
from datetime import date
from pathlib import Path


_CATALOG_RELATIVE = Path("pkg") / "backtest" / "internal" / "stream" / "dependency" / "etf_catalog.json"


@dataclass(frozen=True)
class ETFFund:
    name: str
    url: str
    benchmark: str
    inception: date


_CATALOG: tuple[ETFFund, ...] | None = None


def _catalog_path() -> Path:
    starts = [Path(__file__).resolve(), Path.cwd().resolve()]
    seen: set[Path] = set()
    for start in starts:
        for parent in [start, *start.parents]:
            if parent in seen:
                continue
            seen.add(parent)
            candidate = parent / _CATALOG_RELATIVE
            if candidate.is_file():
                return candidate
    raise FileNotFoundError(f"ETF catalog not found at {_CATALOG_RELATIVE}")


def _load_etf_catalog(path: Path) -> tuple[ETFFund, ...]:
    entries = json.loads(path.read_text())
    if not isinstance(entries, list) or not entries:
        raise ValueError(f"ETF catalog {path} is empty")
    funds: list[ETFFund] = []
    seen: set[str] = set()
    for entry in entries:
        name = str(entry.get("name", "")).strip()
        url = str(entry.get("url", "")).strip()
        benchmark = str(entry.get("benchmark", "")).strip()
        inception_raw = str(entry.get("inception", "")).strip()
        if not name or not url or not benchmark or not inception_raw:
            raise ValueError(f"ETF catalog entry {name!r} is missing name, url, benchmark, or inception")
        if name in seen:
            raise ValueError(f"ETF catalog duplicate name {name!r}")
        seen.add(name)
        funds.append(
            ETFFund(
                name=name,
                url=url,
                benchmark=benchmark,
                inception=date.fromisoformat(inception_raw),
            )
        )
    return tuple(funds)


def etf_catalog() -> tuple[ETFFund, ...]:
    global _CATALOG
    if _CATALOG is None:
        _CATALOG = _load_etf_catalog(_catalog_path())
    return _CATALOG


def etf_by_name(name: str) -> ETFFund | None:
    for fund in etf_catalog():
        if fund.name == name:
            return fund
    return None


def known_etf_names() -> str:
    return ", ".join(fund.name for fund in etf_catalog())
