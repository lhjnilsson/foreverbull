import { createDataFrame, DataFrame, FieldType } from '@grafana/data';

import { ConfigurationView, ExecutionArguments } from './types';

const METADATA_FIELDS = ['backtest', 'benchmark', 'session', 'start', 'end', 'execution'] as const;

function column(name: string, values: string[]) {
  return { name, type: FieldType.string, values };
}

export function metadataFrame(refId: string, execution: ExecutionArguments | null | undefined): DataFrame {
  const row = execution?.id
    ? {
        backtest: execution.backtest ?? '',
        benchmark: execution.benchmark ?? '',
        session: execution.session ?? '',
        start: execution.start ?? '',
        end: execution.end ?? '',
        execution: execution.id,
      }
    : null;
  return createDataFrame({
    refId,
    name: 'Run',
    fields: METADATA_FIELDS.map((name) => column(name, row ? [row[name]] : [])),
  });
}

/** Stored worker arguments only. An empty functions list is zero rows, not template defaults. */
export function argumentsFrame(refId: string, execution: ExecutionArguments | null | undefined): DataFrame {
  const names: string[] = [];
  const keys: string[] = [];
  const values: string[] = [];
  const types: string[] = [];
  for (const fn of execution?.functions ?? []) {
    const params = fn?.parameters ?? [];
    if (params.length === 0) {
      names.push(fn?.name ?? '');
      keys.push('');
      values.push('');
      types.push('');
      continue;
    }
    for (const param of params) {
      names.push(fn?.name ?? '');
      keys.push(param?.key ?? '');
      values.push(param?.value ?? '');
      types.push(param?.value_type ?? '');
    }
  }
  return createDataFrame({
    refId,
    name: 'Worker arguments',
    fields: [
      column('function', names),
      column('key', keys),
      column('value', values),
      column('value_type', types),
    ],
  });
}

export function configurationFrame(
  view: ConfigurationView | undefined,
  refId: string,
  execution: ExecutionArguments | null | undefined
): DataFrame {
  if (view === 'metadata') {
    return metadataFrame(refId, execution);
  }
  return argumentsFrame(refId, execution);
}

export function frameText(frame: DataFrame): string {
  return frame.fields
    .map((field) => {
      const values = typeof field.values.toArray === 'function' ? field.values.toArray() : [];
      return [field.name, ...values].join(' ');
    })
    .join(' ');
}
