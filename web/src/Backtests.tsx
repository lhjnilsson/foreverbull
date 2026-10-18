import { type FormEvent, type ReactNode, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { BacktestFields, EntryForm, Field, TextControl } from "./fields";
import type { Backtest, DateParts, Execution, ExecutionResponse, ExecutionResult } from "./types";
import { postJson, readString, useJson, type Load } from "./useJson";

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

function formatDate(date: DateParts | undefined): string {
  if (!date) return "";
  const { year, month, day } = date;
  if (typeof year !== "number" || typeof month !== "number" || typeof day !== "number") return "";
  if (year === 0 || month === 0 || day === 0) return "";
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

function OpenRun({ id }: { id: string }) {
  return (
    <a className="run-link" href={`/d/ee9ep4jxtwkcgf?var-execution=${encodeURIComponent(id)}`}>
      Open the run.
    </a>
  );
}

function latestResult(execution: Execution, periods?: ExecutionResult[]): ExecutionResult | undefined {
  if (execution.result) return execution.result;
  if (periods && periods.length > 0) return periods[periods.length - 1];
  return undefined;
}

function formatNumber(value: number | undefined): string {
  if (typeof value !== "number" || Number.isNaN(value)) return "";
  return String(value);
}

function ExecutionView({
  execution,
  periods,
}: {
  execution: Execution;
  periods?: ExecutionResult[];
}) {
  const id = execution.id ?? "";
  const result = latestResult(execution, periods);
  const sharpe = formatNumber(result?.sharpe);
  const drawdown = formatNumber(result?.maxDrawdown);
  const endValue = formatNumber(result?.endingValue);
  return (
    <div className="group">
      <Field label="id">{execution.id}</Field>
      <Field label="status">{result ? "stored" : "no result"}</Field>
      <Field label="start">{formatDate(execution.startDate)}</Field>
      <Field label="end">{formatDate(execution.endDate)}</Field>
      {sharpe !== "" ? <Field label="sharpe">{sharpe}</Field> : null}
      {drawdown !== "" ? <Field label="max drawdown">{drawdown}</Field> : null}
      {endValue !== "" ? <Field label="end value">{endValue}</Field> : null}
      {id !== "" ? <OpenRun id={id} /> : null}
    </div>
  );
}

function Executions({ name, revision }: { name: string; revision: number }) {
  const load = useJson<Execution[]>(
    `/api/backtests/${encodeURIComponent(name)}/executions`,
    revision,
  );
  if (load.state === "ready" && Array.isArray(load.data)) {
    if (load.data.length === 0) return null;
    return (
      <div className="executions">
        {load.data.map((execution, index) => (
          <ExecutionView key={`${execution.id ?? ""}-${index}`} execution={execution} />
        ))}
      </div>
    );
  }
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

function StartedRun({ id }: { id: string }) {
  const load = useJson<ExecutionResponse>(`/api/executions/${encodeURIComponent(id)}`);
  const execution = load.state === "ready" ? load.data.execution : undefined;
  const periods = load.state === "ready" ? load.data.periods : undefined;
  const linked = execution?.id !== undefined && execution.id !== "";
  return (
    <>
      {execution ? <ExecutionView execution={execution} periods={periods} /> : null}
      {load.state === "error" && load.message ? <p className="form-error">{load.message}</p> : null}
      {linked ? null : <OpenRun id={id} />}
    </>
  );
}

function RunForm({ name, onStarted }: { name: string; onStarted: () => void }) {
  const [service, setService] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const [runId, setRunId] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setError("");
    setPending(true);
    const result = await postJson(`/api/backtests/${encodeURIComponent(name)}/executions`, {
      service: service.trim(),
    });
    setPending(false);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    const id = readString(result.data, "id");
    if (!id) return;
    setRunId(id);
    onStarted();
  }

  return (
    <>
      <EntryForm
        error={error}
        pending={pending}
        submitLabel="Run this service on the backtest."
        onSubmit={onSubmit}
      >
        <TextControl label="service" value={service} onChange={setService} />
      </EntryForm>
      {runId ? <StartedRun id={runId} /> : null}
    </>
  );
}

function readBacktestName(value: unknown): string | null {
  if (typeof value !== "object" || value === null || !("backtest" in value)) return null;
  return readString((value as { backtest: unknown }).backtest, "name");
}

function CreateBacktest({ onCreated }: { onCreated: (name: string) => void }) {
  const [name, setName] = useState("");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [provider, setProvider] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setError("");
    setPending(true);
    const body: { name: string; start: string; end: string; provider?: string } = {
      name: name.trim(),
      start: start.trim(),
      end: end.trim(),
    };
    // An empty provider is omitted. The server stores yahoo.
    const chosen = provider.trim();
    if (chosen !== "") body.provider = chosen;
    const result = await postJson("/api/backtests", body);
    setPending(false);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    const created = readBacktestName(result.data);
    if (!created) return;
    setName("");
    setStart("");
    setEnd("");
    setProvider("");
    onCreated(created);
  }

  return (
    <EntryForm error={error} pending={pending} submitLabel="Create this backtest." onSubmit={onSubmit}>
      <TextControl label="name" value={name} onChange={setName} />
      <TextControl label="start" type="date" value={start} onChange={setStart} />
      <TextControl label="end" type="date" value={end} onChange={setEnd} />
      <TextControl label="provider" value={provider} onChange={setProvider} />
    </EntryForm>
  );
}

export function Backtests() {
  const { name } = useParams();
  const navigate = useNavigate();
  const [revision, setRevision] = useState(0);
  const [execRevision, setExecRevision] = useState(0);
  const list = useJson<Backtest[]>("/api/backtests", revision);
  const detailUrl = name === undefined ? null : `/api/backtests/${encodeURIComponent(name)}`;
  const detail = useJson<Backtest>(detailUrl, revision);

  return (
    <Area
      list={<BacktestList load={list} selected={name} />}
      detail={
        <>
          <BacktestDetail load={detail} />
          {name ? (
            <>
              <Executions key={name} name={name} revision={execRevision} />
              <RunForm key={name} name={name} onStarted={() => setExecRevision((current) => current + 1)} />
            </>
          ) : null}
          <CreateBacktest
            onCreated={(created) => {
              setRevision((current) => current + 1);
              navigate(`/backtests/${encodeURIComponent(created)}`);
            }}
          />
        </>
      }
    />
  );
}
