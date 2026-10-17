import type { ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { BacktestFields } from "./fields";
import type { Backtest } from "./types";
import { useJson, type Load } from "./useJson";

function Area({ list, detail }: { list: ReactNode; detail: ReactNode }) {
  return (
    <div className="split">
      <div className="list">{list}</div>
      <div className="detail">{detail}</div>
    </div>
  );
}

function BacktestRow({ backtest, selected }: { backtest: Backtest; selected: boolean }) {
  const name = backtest.name ?? "";
  return (
    <Link
      className={selected ? "row selected" : "row"}
      to={`/backtests/${encodeURIComponent(name)}`}
      aria-current={selected ? "page" : undefined}
    >
      <span className="primary">{name}</span>
      {backtest.provider ? <span className="secondary">{backtest.provider}</span> : null}
    </Link>
  );
}

function BacktestList({ load, selected }: { load: Load<Backtest[]>; selected: string | undefined }) {
  if (load.state === "ready" && Array.isArray(load.data)) {
    if (load.data.length === 0) return <p className="empty">Create a backtest.</p>;
    return (
      <>
        {load.data.map((backtest, index) => (
          <BacktestRow
            key={`${backtest.name ?? ""}-${index}`}
            backtest={backtest}
            selected={backtest.name !== undefined && backtest.name === selected}
          />
        ))}
      </>
    );
  }
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

function BacktestDetail({ load }: { load: Load<Backtest> }) {
  if (load.state === "ready" && load.data) return <BacktestFields backtest={load.data} />;
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

export function Backtests() {
  const { name } = useParams();
  const list = useJson<Backtest[]>("/api/backtests");
  const detailUrl = name === undefined ? null : `/api/backtests/${encodeURIComponent(name)}`;
  const detail = useJson<Backtest>(detailUrl);

  return (
    <Area
      list={<BacktestList load={list} selected={name} />}
      detail={<BacktestDetail load={detail} />}
    />
  );
}
