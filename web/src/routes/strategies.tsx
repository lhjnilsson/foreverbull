import { type FormEvent, useState } from "react";
import { Link, Navigate, Outlet, useMatch, useNavigate, useOutletContext } from "react-router-dom";
import {
  Detail,
  Empty,
  Field,
  FormError,
  ListPane,
  buttonClass,
  fmtTime,
  inputClass,
  rowActive,
  rowClass,
} from "../components/fb";
import type { Strategy } from "../types";
import { postJson, readString, useJson, type Load } from "../useJson";

type StrategyList = {
  load: Load<Strategy[]>;
  refresh: () => void;
};

function rowsOf(load: Load<Strategy[]>): Strategy[] | null {
  if (load.state !== "ready" || !Array.isArray(load.data)) return null;
  return load.data;
}

export function StrategiesLayout() {
  const navigate = useNavigate();
  const match = useMatch("/strategies/:name");
  const name = match?.params.name;
  const [revision, setRevision] = useState(0);
  const load = useJson<Strategy[]>("/api/strategies", revision);
  const rows = rowsOf(load);
  const refresh = () => setRevision((current) => current + 1);
  const list: StrategyList = { load, refresh };

  return (
    <>
      <ListPane
        title="Strategies"
        newLabel="New strategy"
        newTitle="Name this strategy."
        newForm={(close) => (
          <CreateStrategy
            onDone={close}
            onCreated={(created) => {
              refresh();
              navigate(`/strategies/${encodeURIComponent(created)}`);
            }}
          />
        )}
      >
        {load.state === "error" && load.message ? (
          <li className="px-4 py-6 text-muted-foreground">{load.message}</li>
        ) : null}
        {rows && rows.length === 0 ? <Empty text="Name a strategy." /> : null}
        {(rows ?? []).map((strategy) => (
          <li key={strategy.id ?? strategy.name}>
            <Link
              to={strategy.name ? `/strategies/${encodeURIComponent(strategy.name)}` : "/strategies"}
              className={`${rowClass} font-mono text-[13px] ${strategy.name === name ? rowActive : ""}`}
            >
              {strategy.name}
            </Link>
          </li>
        ))}
      </ListPane>
      <Outlet context={list satisfies StrategyList} />
    </>
  );
}

export function StrategyIndex() {
  const { load } = useOutletContext<StrategyList>();
  const rows = rowsOf(load);
  const first = rows?.find((strategy) => strategy.name)?.name;
  if (first) return <Navigate to={`/strategies/${encodeURIComponent(first)}`} replace />;
  return (
    <Detail k="none">
      <p className="text-muted-foreground">Select a strategy.</p>
    </Detail>
  );
}

export function StrategyDetail() {
  const match = useMatch("/strategies/:name");
  const name = match?.params.name ?? "";
  const { load } = useOutletContext<StrategyList>();
  const listed = rowsOf(load)?.find((item) => item.name === name);
  const detail = useJson<Strategy>(name ? `/api/strategies/${encodeURIComponent(name)}` : null);
  if (detail.state === "error") {
    return (
      <Detail k="missing">
        <p className="text-muted-foreground">
          {detail.message === "not found" || !detail.message ? `No strategy named ${name}.` : detail.message}
        </p>
      </Detail>
    );
  }
  const strategy = detail.state === "ready" ? detail.data : listed;
  if (!strategy?.name) {
    return <Detail k={name || "loading"} />;
  }
  return (
    <Detail k={strategy.id ?? strategy.name ?? name}>
      <h1 className="mb-4 font-mono text-base">{strategy.name}</h1>
      <dl>
        <Field label="Name">{strategy.name}</Field>
        <Field label="Created">{fmtTime(strategy.createdAt)}</Field>
      </dl>
    </Detail>
  );
}

function CreateStrategy({ onDone, onCreated }: { onDone: () => void; onCreated: (name: string) => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (pending || !trimmed) return;
    setError("");
    setPending(true);
    const result = await postJson("/api/strategies", { name: trimmed });
    setPending(false);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    const created = readString(result.data, "name") ?? trimmed;
    setName("");
    onCreated(created);
    onDone();
  }

  return (
    <form className="max-w-md" onSubmit={onSubmit}>
      <label className="mb-1 block text-muted-foreground" htmlFor="name">
        Name
      </label>
      <input
        id="name"
        className={inputClass}
        value={name}
        spellCheck={false}
        autoComplete="off"
        onChange={(event) => setName(event.target.value)}
      />
      <button className={`${buttonClass} mt-4`} disabled={!name.trim() || pending}>
        Name this strategy.
      </button>
      <FormError message={error} />
    </form>
  );
}
