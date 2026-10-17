import type { ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { StrategyFields } from "./fields";
import type { Strategy } from "./types";
import { useJson, type Load } from "./useJson";

function Area({ list, detail }: { list: ReactNode; detail: ReactNode }) {
  return (
    <div className="split">
      <div className="list">{list}</div>
      <div className="detail">{detail}</div>
    </div>
  );
}

function StrategyRow({ strategy, selected }: { strategy: Strategy; selected: boolean }) {
  const name = strategy.name ?? "";
  return (
    <Link
      className={selected ? "row selected" : "row"}
      to={`/strategies/${encodeURIComponent(name)}`}
      aria-current={selected ? "page" : undefined}
    >
      <span className="primary">{name}</span>
      {strategy.id ? <span className="secondary">{strategy.id}</span> : null}
    </Link>
  );
}

function StrategyList({ load, selected }: { load: Load<Strategy[]>; selected: string | undefined }) {
  if (load.state === "ready" && Array.isArray(load.data)) {
    if (load.data.length === 0) return <p className="empty">Name a strategy.</p>;
    return (
      <>
        {load.data.map((strategy, index) => (
          <StrategyRow
            key={`${strategy.name ?? ""}-${index}`}
            strategy={strategy}
            selected={strategy.name !== undefined && strategy.name === selected}
          />
        ))}
      </>
    );
  }
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

function StrategyDetail({ load }: { load: Load<Strategy> }) {
  if (load.state === "ready" && load.data) return <StrategyFields strategy={load.data} />;
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

export function Strategies() {
  const { name } = useParams();
  const list = useJson<Strategy[]>("/api/strategies");
  const detailUrl = name === undefined ? null : `/api/strategies/${encodeURIComponent(name)}`;
  const detail = useJson<Strategy>(detailUrl);

  return (
    <Area
      list={<StrategyList load={list} selected={name} />}
      detail={<StrategyDetail load={detail} />}
    />
  );
}
