import { type FormEvent, type ReactNode, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { EntryForm, StrategyFields, TextControl } from "./fields";
import type { Strategy } from "./types";
import { postJson, readString, useJson, type Load } from "./useJson";

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

function NameStrategy({ onCreated }: { onCreated: (name: string) => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setError("");
    setPending(true);
    const result = await postJson("/api/strategies", { name: name.trim() });
    setPending(false);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    const created = readString(result.data, "name");
    if (!created) return;
    setName("");
    onCreated(created);
  }

  return (
    <EntryForm error={error} pending={pending} submitLabel="Name this strategy." onSubmit={onSubmit}>
      <TextControl label="name" value={name} onChange={setName} />
    </EntryForm>
  );
}

export function Strategies() {
  const { name } = useParams();
  const navigate = useNavigate();
  const [revision, setRevision] = useState(0);
  const list = useJson<Strategy[]>("/api/strategies", revision);
  const detailUrl = name === undefined ? null : `/api/strategies/${encodeURIComponent(name)}`;
  const detail = useJson<Strategy>(detailUrl, revision);

  return (
    <Area
      list={<StrategyList load={list} selected={name} />}
      detail={
        <>
          <StrategyDetail load={detail} />
          <NameStrategy
            onCreated={(created) => {
              setRevision((current) => current + 1);
              navigate(`/strategies/${encodeURIComponent(created)}`);
            }}
          />
        </>
      }
    />
  );
}
