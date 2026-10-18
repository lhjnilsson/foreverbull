import type { FormEvent, ReactNode } from "react";
import type {
  Algorithm,
  AlgorithmFunction,
  Backtest,
  BacktestAsset,
  BacktestPeriod,
  DateParts,
  FunctionParameter,
  Service,
  ServiceStatus,
  Strategy,
} from "./types";

export function Field({
  label,
  children,
  prose,
}: {
  label: string;
  children: ReactNode;
  prose?: boolean;
}) {
  return (
    <div className="field">
      <div className="label">{label}</div>
      <div className={prose ? "value prose" : "value"}>{children}</div>
    </div>
  );
}

export function TextControl({
  label,
  value,
  onChange,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: "text" | "date";
}) {
  return (
    <label className="create-field">
      <span className="label">{label}</span>
      <input
        className="control"
        type={type}
        value={value}
        spellCheck={false}
        autoComplete="off"
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

export function EntryForm({
  error,
  pending,
  submitLabel,
  onSubmit,
  children,
}: {
  error: string;
  pending: boolean;
  submitLabel: string;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  children: ReactNode;
}) {
  return (
    <form className="form" onSubmit={onSubmit}>
      {children}
      {error ? <p className="form-error">{error}</p> : null}
      <button className="action" type="submit" disabled={pending}>
        {submitLabel}
      </button>
    </form>
  );
}

export function boolText(value: boolean | undefined): string {
  if (value === undefined) return "";
  return value ? "true" : "false";
}

function statusTone(status: string | undefined): string {
  if (status === "READY") return "status-ok";
  if (status === "ERROR") return "status-fail";
  if (status === "CREATED" || status === "INTERVIEW") return "status-warn";
  return "";
}

export function StatusText({ status }: { status: string | undefined }) {
  if (!status) return null;
  const tone = statusTone(status);
  return <span className={tone ? `status-text ${tone}` : "status-text"}>{status}</span>;
}

function DateFields({ date }: { date: DateParts | undefined }) {
  if (!date) return null;
  return (
    <div className="nested">
      <Field label="year">{date.year}</Field>
      <Field label="month">{date.month}</Field>
      <Field label="day">{date.day}</Field>
    </div>
  );
}

function AssetFields({ asset }: { asset: BacktestAsset }) {
  return (
    <div className="group">
      <Field label="symbol">{asset.symbol}</Field>
      <Field label="enabled">{boolText(asset.enabled)}</Field>
    </div>
  );
}

function PeriodFields({ period }: { period: BacktestPeriod }) {
  return (
    <div className="group">
      <Field label="startDate">
        <DateFields date={period.startDate} />
      </Field>
      <Field label="endDate">
        <DateFields date={period.endDate} />
      </Field>
      <Field label="assets">
        <div className="nested">
          {(period.assets ?? []).map((asset, index) => (
            <AssetFields key={`${asset.symbol ?? ""}-${index}`} asset={asset} />
          ))}
        </div>
      </Field>
    </div>
  );
}

export function BacktestFields({ backtest }: { backtest: Backtest }) {
  return (
    <div className="fields">
      <Field label="name">{backtest.name}</Field>
      <Field label="benchmark">{backtest.benchmark}</Field>
      <Field label="periods">
        <div className="nested">
          {(backtest.periods ?? []).map((period, index) => (
            <PeriodFields key={index} period={period} />
          ))}
        </div>
      </Field>
      <Field label="provider">{backtest.provider}</Field>
      <Field label="profile">{backtest.profile}</Field>
    </div>
  );
}

export function StrategyFields({ strategy }: { strategy: Strategy }) {
  return (
    <div className="fields">
      <Field label="id">{strategy.id}</Field>
      <Field label="name">{strategy.name}</Field>
      <Field label="description" prose>
        {strategy.description}
      </Field>
      <Field label="createdAt">{strategy.createdAt}</Field>
    </div>
  );
}

function ParameterFields({ parameter }: { parameter: FunctionParameter }) {
  return (
    <div className="group">
      <Field label="key">{parameter.key}</Field>
      <Field label="defaultValue">{parameter.defaultValue}</Field>
      <Field label="valueType">{parameter.valueType}</Field>
    </div>
  );
}

function FunctionFields({ fn }: { fn: AlgorithmFunction }) {
  return (
    <div className="group">
      <Field label="name">{fn.name}</Field>
      <Field label="parameters">
        <div className="nested">
          {(fn.parameters ?? []).map((parameter, index) => (
            <ParameterFields key={`${parameter.key ?? ""}-${index}`} parameter={parameter} />
          ))}
        </div>
      </Field>
      <Field label="parallelExecution">{boolText(fn.parallelExecution)}</Field>
      <Field label="runFirst">{boolText(fn.runFirst)}</Field>
      <Field label="runLast">{boolText(fn.runLast)}</Field>
    </div>
  );
}

function AlgorithmFields({ algorithm }: { algorithm: Algorithm | undefined }) {
  if (!algorithm) return null;
  return (
    <div className="nested">
      <Field label="filePath">{algorithm.filePath}</Field>
      <Field label="functions">
        <div className="nested">
          {(algorithm.functions ?? []).map((fn, index) => (
            <FunctionFields key={`${fn.name ?? ""}-${index}`} fn={fn} />
          ))}
        </div>
      </Field>
      <Field label="namespaces">
        <ul className="tokens">
          {(algorithm.namespaces ?? []).map((ns) => (
            <li key={ns}>{ns}</li>
          ))}
        </ul>
      </Field>
    </div>
  );
}

function StatusFields({ status }: { status: ServiceStatus }) {
  return (
    <div className="group">
      <Field label="status">
        <StatusText status={status.status} />
      </Field>
      <Field label="OccurredAt">{status.OccurredAt}</Field>
    </div>
  );
}

export function ServiceFields({ service }: { service: Service }) {
  return (
    <div className="fields">
      <Field label="Image">{service.Image}</Field>
      <Field label="algorithm">
        <AlgorithmFields algorithm={service.algorithm} />
      </Field>
      <Field label="statuses">
        <div className="nested">
          {(service.statuses ?? []).map((status, index) => (
            <StatusFields key={`${status.status ?? ""}-${status.OccurredAt ?? ""}-${index}`} status={status} />
          ))}
        </div>
      </Field>
    </div>
  );
}
