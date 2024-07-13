from typing import List

from foreverbull import entity

from .http import Session, inject_session


@inject_session
def ingest(session: Session, ingestion: entity.backtest.Ingestion) -> entity.backtest.Ingestion:
    rsp = session.request("POST", "/backtest/api/ingestion", json=ingestion.model_dump())
    return entity.backtest.Ingestion.parse_obj(rsp.json())


@inject_session
def get_ingestion(session: Session) -> entity.backtest.Ingestion:
    rsp = session.request("GET", "/backtest/api/ingestion")
    return entity.backtest.Ingestion.parse_obj(rsp.json())


@inject_session
def list(session: Session) -> List[entity.backtest.Backtest]:
    rsp = session.request("GET", "/backtest/api/backtests")
    return [entity.backtest.Backtest.parse_obj(b) for b in rsp.json()]


@inject_session
def create(session: Session, backtest: entity.backtest.Backtest) -> entity.backtest.Backtest:
    rsp = session.request("POST", "/backtest/api/backtests", json=backtest.model_dump())
    return entity.backtest.Backtest.parse_obj(rsp.json())


@inject_session
def get(session: Session, name: str) -> entity.backtest.Backtest:
    rsp = session.request("GET", f"/backtest/api/backtests/{name}")
    return entity.backtest.Backtest.parse_obj(rsp.json())


@inject_session
def list_sessions(session: Session, backtest: str | None = None) -> List[entity.backtest.Session]:
    rsp = session.request("GET", "/backtest/api/sessions", params={"backtest": backtest})
    return [entity.backtest.Session.parse_obj(s) for s in rsp.json()]


@inject_session
def run(session: Session, backtest: str, manual: bool = False) -> entity.backtest.Session:
    rsp = session.request("POST", "/backtest/api/sessions", json={"backtest": backtest, "manual": manual})
    return entity.backtest.Session.parse_obj(rsp.json())


@inject_session
def get_session(session: Session, session_id: str) -> entity.backtest.Session:
    rsp = session.request("GET", f"/backtest/api/sessions/{session_id}")
    return entity.backtest.Session.parse_obj(rsp.json())


@inject_session
def list_executions(s: Session, session: str | None = None) -> List[entity.backtest.Execution]:
    rsp = s.request("GET", "/backtest/api/executions", params={"session": session})
    return [entity.backtest.Execution.parse_obj(e) for e in rsp.json()]


@inject_session
def get_execution(session: Session, execution_id: str) -> entity.backtest.Execution:
    rsp = session.request("GET", f"/backtest/api/executions/{execution_id}")
    return entity.backtest.Execution.parse_obj(rsp.json())
