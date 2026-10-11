import { MetricFindValue } from '@grafana/data';

import { concreteExecutionIds, idsFromLocationSearch } from './executionRange';
import { variableSource } from './query';
import {
  ExecutionArguments,
  ExecutionLabelStyle,
  MyQuery,
  ResourceDefinition,
  ResourceFilter,
  ResourcePath,
} from './types';

/** Backtest name captured from the page URL before Grafana fills var-backtest. */
export interface DashboardEntry {
  path: string;
  executionId: string;
  hadBacktest: boolean;
}

export interface ExecutionPin {
  backtest: string;
  /** True once a query's filter was already this backtest. A later change is the user's. */
  acknowledged: boolean;
}

export interface ExecutionVariableState {
  entryExecutionId: string;
  entryHadBacktest: boolean;
  pin: ExecutionPin | null;
}

export interface ExecutionVariableClient {
  getExecutions(filter?: ResourceFilter): Promise<ResourceDefinition[]>;
  getExecutionArguments(id: string): Promise<ExecutionArguments | null>;
}

export interface ExecutionVariableResult {
  state: ExecutionVariableState;
  /** Write var-backtest when the deep link had none. */
  setBacktest?: string;
  items: ResourceDefinition[];
  /** Present when the id was missing from ListExecutions and had to be kept. */
  injected?: ResourceDefinition;
  options: MetricFindValue[];
}

const DATE_SEPARATOR = '–';

export function concreteBacktest(value?: string | null): string {
  const text = (value ?? '').trim();
  if (!text || text.includes('$') || text === 'All' || text === 'None' || text === '$__all') {
    return '';
  }
  return text;
}

function searchText(value: unknown): string {
  if (Array.isArray(value)) {
    return searchText(value[0]);
  }
  if (typeof value === 'string' || typeof value === 'number') {
    return String(value);
  }
  return '';
}

export function dashboardEntry(path: string, search: Record<string, unknown> | null | undefined): DashboardEntry {
  const source = search ?? {};
  return {
    path,
    executionId: idsFromLocationSearch(source)[0] ?? '',
    hadBacktest: concreteBacktest(searchText(source['var-backtest'])) !== '',
  };
}

/** Keep the entry taken on first load. A new ?var-execution= link (no var-backtest) starts over. */
export function shouldResetExecutionEntry(prev: DashboardEntry | null, next: DashboardEntry): boolean {
  if (!prev || prev.path !== next.path) {
    return true;
  }
  return next.executionId !== '' && !next.hadBacktest && next.executionId !== prev.executionId;
}

/** Execution dashboard variable: backtest names cascade into this list. Compare pickers do not. */
export function usesExecutionBacktestSelector(query: Partial<MyQuery> | undefined): boolean {
  if (!query || variableSource(query) !== ResourcePath.Executions) {
    return false;
  }
  if (query.label === 'dates') {
    return true;
  }
  return Boolean(query.backtest && query.backtest.includes('$backtest'));
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

/** `dates` is start–end and a short id. The backtest name stays on its own selector. */
export function executionPickerLabel(item: ResourceDefinition, style: ExecutionLabelStyle): string {
  if (style !== 'dates') {
    return item.label || item.value;
  }
  const parts: string[] = [];
  if (item.start && item.end) {
    parts.push(`${item.start}${DATE_SEPARATOR}${item.end}`);
  } else if (item.start) {
    parts.push(item.start);
  } else if (item.end) {
    parts.push(item.end);
  }
  if (item.value) {
    parts.push(shortId(item.value));
  }
  return parts.join(' ') || item.label || item.value;
}

export function toExecutionPickerValues(items: ResourceDefinition[], style: ExecutionLabelStyle): MetricFindValue[] {
  return items
    .filter((item) => item.value)
    .map((item) => ({
      text: executionPickerLabel(item, style),
      value: item.value,
    }));
}

async function lookupExecution(client: ExecutionVariableClient, id: string): Promise<ExecutionArguments | null> {
  try {
    const view = await client.getExecutionArguments(id);
    return view?.id ? view : null;
  } catch {
    return null;
  }
}

/**
 * Lists executions for the selected backtest.
 * A deep link that only has var-execution is resolved from that run's backtest name,
 * and the id stays in the options even when the backtest variable was still empty.
 */
export async function loadExecutionVariableOptions(
  client: ExecutionVariableClient,
  state: ExecutionVariableState | null | undefined,
  requestedBacktest: string | undefined,
  label: ExecutionLabelStyle
): Promise<ExecutionVariableResult> {
  const current: ExecutionVariableState = {
    entryExecutionId: state?.entryExecutionId ?? '',
    entryHadBacktest: Boolean(state?.entryHadBacktest),
    pin: state?.pin ? { ...state.pin } : null,
  };
  const requested = concreteBacktest(requestedBacktest);
  const entryId = concreteExecutionIds([current.entryExecutionId])[0] ?? '';
  const deepLink = Boolean(entryId) && !current.entryHadBacktest;

  let setBacktest: string | undefined;
  let lookedUp: ExecutionArguments | null = null;

  if (deepLink && !current.pin) {
    lookedUp = await lookupExecution(client, entryId);
    const name = concreteBacktest(lookedUp?.backtest);
    if (name) {
      current.pin = { backtest: name, acknowledged: requested === name };
      if (requested !== name) {
        setBacktest = name;
      }
    }
  } else if (deepLink && current.pin && !current.pin.acknowledged && requested === current.pin.backtest) {
    current.pin = { ...current.pin, acknowledged: true };
  }

  let backtest = requested;
  if (deepLink && current.pin && !current.pin.acknowledged) {
    backtest = current.pin.backtest;
  }

  const filter: ResourceFilter = {};
  if (backtest) {
    filter.backtest = backtest;
  }
  const items = [...(await client.getExecutions(filter))];

  let injected: ResourceDefinition | undefined;
  const listingPinnedBacktest = !current.pin || backtest === current.pin.backtest;
  if (deepLink && listingPinnedBacktest && entryId && !items.some((item) => item.value === entryId)) {
    if (!lookedUp) {
      lookedUp = await lookupExecution(client, entryId);
    }
    injected = {
      value: entryId,
      label: shortId(entryId),
      start: lookedUp?.start,
      end: lookedUp?.end,
    };
    items.unshift(injected);
  }

  return {
    state: current,
    setBacktest,
    items,
    injected,
    options: toExecutionPickerValues(items, label),
  };
}
