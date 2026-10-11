/** Epoch milliseconds. Grafana URL `from`/`to` use this, not `now` or `now-30y`. */
export interface EpochRange {
  fromMs: number;
  toMs: number;
}

export interface RangeDecision {
  /** When false, leave the current dashboard window alone. */
  apply: boolean;
  /** Sorted concrete execution ids. A new id must replace the window. */
  executionKey: string;
  /** Epoch ms string for locationService. Absent when apply is false. */
  from?: string;
  to?: string;
}

/** catalog = ListExecutions start/end. series = GetExecutionMetric period timestamps. */
export type SpanSource = 'catalog' | 'series';

export interface RangeDecisionInput {
  executionIds: Array<string | undefined | null>;
  /** Min/max timestamps for those executions. Null when no time is known yet. */
  span: EpochRange | null;
  spanSource?: SpanSource;
  /** Id key we already applied. Same key must not snap a zoom back on refresh. */
  appliedExecutionKey: string | null;
  /** Range last written for that key. Series min/max replaces it when they differ. */
  appliedRange?: EpochRange | null;
  /**
   * Dashboard window at decision time, including now/now or a zoom.
   * Not a gate: first entry applies even when this window misses the points.
   */
  current: EpochRange | null;
}

export interface ExecutionSpanOption {
  id: string;
  /** YYYY-MM-DD or epoch ms. ListExecutions start_date. */
  start?: string;
  /** YYYY-MM-DD or epoch ms. ListExecutions end_date. */
  end?: string;
}

const DASHBOARD_EXECUTION_VARIABLES = ['execution', 'left', 'right'];

/** True when `current` sits inside `span`. Informational; it does not gate apply. */
export function rangeIsInside(current: EpochRange, span: EpochRange): boolean {
  return current.fromMs >= span.fromMs && current.toMs <= span.toMs && current.toMs > current.fromMs;
}

export function concreteExecutionIds(ids: Array<string | undefined | null>): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of ids) {
    const id = (raw ?? '').trim();
    if (!id || id.includes('$') || id === 'All' || id === 'None' || id === '$__all') {
      continue;
    }
    if (seen.has(id)) {
      continue;
    }
    seen.add(id);
    out.push(id);
  }
  return out;
}

export function executionKey(ids: Array<string | undefined | null>): string {
  return concreteExecutionIds(ids).sort().join('|');
}

/**
 * Panel queries still carry `$execution` when query() runs. Replace it, and if that
 * is still a placeholder, use `var-execution` from the URL.
 */
export function resolveExecutionIds(
  targetIds: Array<string | undefined | null>,
  replace: (value: string) => string,
  locationSearch?: Record<string, unknown> | null
): string[] {
  const replaced = targetIds.map((id) => (id ? replace(id) : id));
  const concrete = concreteExecutionIds(replaced);
  if (concrete.length > 0) {
    return concrete;
  }
  return idsFromLocationSearch(locationSearch);
}

/**
 * Decide whether to move the dashboard time axis.
 * A new execution id gets that run's exact min and max (no pad). The same id
 * keeps a later zoom. Series timestamps replace a catalog range when they differ.
 */
export function decideExecutionTimeRange(input: RangeDecisionInput): RangeDecision {
  const key = executionKey(input.executionIds);
  if (!key || !input.span || !Number.isFinite(input.span.fromMs) || !Number.isFinite(input.span.toMs)) {
    return { apply: false, executionKey: key };
  }

  const next = roundedSpan(input.span);
  if (input.appliedExecutionKey === key) {
    // Same run. Series min/max may still replace ListExecutions dates. A zoom stays.
    if (input.spanSource === 'series' && input.appliedRange && !sameRange(input.appliedRange, next)) {
      return applied(key, next);
    }
    // The picker can still say "Last 30 years" after an earlier apply was overwritten.
    // A window that is not this run, and not a zoom inside it, is set again.
    if (
      input.current &&
      !sameRange(input.current, next) &&
      !rangeIsInside(input.current, next)
    ) {
      return applied(key, next);
    }
    return { apply: false, executionKey: key };
  }

  // First entry, or a different id. Apply even if `current` is now/now or a zoom inside the new span.
  return applied(key, next);
}

function applied(key: string, span: EpochRange): RangeDecision {
  return {
    apply: true,
    executionKey: key,
    from: String(span.fromMs),
    to: String(span.toMs),
  };
}

function roundedSpan(span: EpochRange): EpochRange {
  const normalized = normalizeSpan(span);
  return { fromMs: Math.round(normalized.fromMs), toMs: Math.round(normalized.toMs) };
}

function sameRange(left: EpochRange, right: EpochRange): boolean {
  const a = roundedSpan(left);
  const b = roundedSpan(right);
  return a.fromMs === b.fromMs && a.toMs === b.toMs;
}

function normalizeSpan(span: EpochRange): EpochRange {
  if (span.toMs < span.fromMs) {
    return { fromMs: span.toMs, toMs: span.fromMs };
  }
  return span;
}

export interface TimeValueSource {
  length: number;
  get?: (index: number) => unknown;
  [index: number]: unknown;
}

export interface TimeFieldLike {
  name?: string;
  type?: string;
  values?: TimeValueSource;
  labels?: Record<string, string>;
}

export interface FrameLike {
  fields?: TimeFieldLike[];
}

const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/;

export function toEpochMs(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (value instanceof Date) {
    const ms = value.getTime();
    return Number.isFinite(ms) ? ms : null;
  }
  if (typeof value === 'string') {
    const dateOnly = DATE_ONLY.exec(value);
    if (dateOnly) {
      const year = Number(dateOnly[1]);
      const month = Number(dateOnly[2]);
      const day = Number(dateOnly[3]);
      if (month < 1 || month > 12 || day < 1 || day > 31) {
        return null;
      }
      return Date.UTC(year, month - 1, day);
    }
    if (/^\d+$/.test(value)) {
      const ms = Number(value);
      return Number.isFinite(ms) ? ms : null;
    }
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  if (value && typeof value === 'object' && 'valueOf' in value) {
    const raw = (value as { valueOf: () => unknown }).valueOf();
    if (raw !== value && typeof raw === 'number' && Number.isFinite(raw)) {
      return raw;
    }
  }
  return null;
}

function isTimeField(field: TimeFieldLike): boolean {
  return field.type === 'time' || field.name === 'time';
}

function valueAt(values: TimeValueSource, index: number): unknown {
  if (typeof values.get === 'function') {
    return values.get(index);
  }
  return values[index];
}

/** Union of period timestamps across frames (one execution, or a left/right overlay). */
export function spanFromFrames(frames: FrameLike[] | undefined | null): EpochRange | null {
  let fromMs = Number.POSITIVE_INFINITY;
  let toMs = Number.NEGATIVE_INFINITY;
  let found = false;
  for (const frame of frames ?? []) {
    for (const field of frame.fields ?? []) {
      if (!isTimeField(field) || !field.values) {
        continue;
      }
      const length = field.values.length ?? 0;
      for (let i = 0; i < length; i++) {
        const ms = toEpochMs(valueAt(field.values, i));
        if (ms == null) {
          continue;
        }
        found = true;
        if (ms < fromMs) {
          fromMs = ms;
        }
        if (ms > toMs) {
          toMs = ms;
        }
      }
    }
  }
  if (!found) {
    return null;
  }
  return { fromMs, toMs };
}

/** Union of ListExecutions start/end for the selected ids. No pad. */
export function spanFromExecutionOptions(
  options: ExecutionSpanOption[] | undefined | null,
  selectedIds: Array<string | undefined | null>
): EpochRange | null {
  const wanted = new Set(concreteExecutionIds(selectedIds));
  if (wanted.size === 0) {
    return null;
  }
  let fromMs = Number.POSITIVE_INFINITY;
  let toMs = Number.NEGATIVE_INFINITY;
  let found = false;
  for (const option of options ?? []) {
    if (!option?.id || !wanted.has(option.id)) {
      continue;
    }
    const start = toEpochMs(option.start);
    const end = toEpochMs(option.end);
    if (start == null || end == null) {
      continue;
    }
    const span = normalizeSpan({ fromMs: start, toMs: end });
    found = true;
    if (span.fromMs < fromMs) {
      fromMs = span.fromMs;
    }
    if (span.toMs > toMs) {
      toMs = span.toMs;
    }
  }
  if (!found) {
    return null;
  }
  return { fromMs, toMs };
}

export function idsFromVariableValues(
  variables: Array<{ name?: string; value?: unknown }> | undefined | null
): string[] {
  const raw: Array<string | undefined | null> = [];
  for (const variable of variables ?? []) {
    if (!variable.name || !DASHBOARD_EXECUTION_VARIABLES.includes(variable.name)) {
      continue;
    }
    pushRaw(raw, variable.value);
  }
  return concreteExecutionIds(raw);
}

export function idsFromLocationSearch(search: Record<string, unknown> | undefined | null): string[] {
  const raw: Array<string | undefined | null> = [];
  const source = search ?? {};
  for (const name of DASHBOARD_EXECUTION_VARIABLES) {
    pushRaw(raw, source[`var-${name}`]);
  }
  return concreteExecutionIds(raw);
}

function pushRaw(out: Array<string | undefined | null>, value: unknown): void {
  if (Array.isArray(value)) {
    for (const item of value) {
      pushRaw(out, item);
    }
    return;
  }
  if (typeof value === 'string' || typeof value === 'number') {
    out.push(String(value));
  }
}
