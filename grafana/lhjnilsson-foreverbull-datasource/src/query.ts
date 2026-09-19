import { MetricFindValue } from '@grafana/data';

import {
  Execution,
  Metric,
  MyQuery,
  QueryType,
  ResourceDefinition,
  ResourceFilter,
  ResourcePath,
  VariableSource,
} from './types';

export function executionRefId(execution?: { id?: string; ID?: string } | null): string {
  if (!execution) {
    return '';
  }
  return execution.id || execution.ID || '';
}

export function toExecution(id: string | undefined): Execution | undefined {
  return id ? { id } : undefined;
}

export function toMetrics(names: Array<string | undefined | null>): Metric[] {
  return names.filter((name): name is string => Boolean(name)).map((name) => ({ name }));
}

export function withExecutionMetric(query: MyQuery, patch: Partial<MyQuery>): MyQuery {
  return {
    ...query,
    ...patch,
    queryType: patch.queryType || query.queryType || QueryType.GetExecutionMetric,
  };
}

export function applyExecutionTemplates(query: MyQuery, replace: (value: string) => string): MyQuery {
  return withExecutionMetric(query, {
    execution: { id: replace(executionRefId(query.execution)) },
  });
}

export function isRunnableQuery(query: MyQuery): boolean {
  return Boolean(executionRefId(query.execution) && query.metrics?.length);
}

export function variableSource(query: Partial<MyQuery> | undefined): VariableSource {
  return query?.source || ResourcePath.Executions;
}

export function variableDefinition(query: Partial<MyQuery>): string {
  const source = variableSource(query);
  const filters = [query.strategy, query.version, query.backtest].filter((v) => v && v.length > 0);
  return filters.length ? `${source} (${filters.join(', ')})` : source;
}

export function resourceFilter(query: Partial<MyQuery>): ResourceFilter {
  return {
    strategy: query.strategy,
    version: query.version,
    backtest: query.backtest,
  };
}

export function toMetricFindValues(items: ResourceDefinition[]): MetricFindValue[] {
  return items
    .filter((item) => item.value)
    .map((item) => ({
      text: item.label || item.value,
      value: item.value,
    }));
}

type VariableResourceClient = {
  getExecutions: (filter?: ResourceFilter) => Promise<ResourceDefinition[]>;
  getStrategies: () => Promise<ResourceDefinition[]>;
  getVersions: (filter?: ResourceFilter) => Promise<ResourceDefinition[]>;
  getExperiments: (filter?: ResourceFilter) => Promise<ResourceDefinition[]>;
};

export async function listVariableOptions(
  ds: VariableResourceClient,
  query: Partial<MyQuery>
): Promise<MetricFindValue[]> {
  const filter = resourceFilter(query);
  let items: ResourceDefinition[];
  switch (variableSource(query)) {
    case ResourcePath.Strategies:
      items = await ds.getStrategies();
      break;
    case ResourcePath.Versions:
      items = await ds.getVersions(filter);
      break;
    case ResourcePath.Experiments:
      items = await ds.getExperiments(filter);
      break;
    default:
      items = await ds.getExecutions(filter);
  }
  return toMetricFindValues(items);
}
