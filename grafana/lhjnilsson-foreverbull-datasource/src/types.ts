import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export enum QueryType {
  GetExecutionMetric = 'GetExecutionMetric',
}

/** Backend resource routes. Execution and experiment values are execution UUIDs. */
export const ResourcePath = {
  Executions: 'executions',
  Metrics: 'metrics',
  Strategies: 'strategies',
  Versions: 'versions',
  Experiments: 'experiments',
} as const;

export type VariableSource =
  | typeof ResourcePath.Executions
  | typeof ResourcePath.Strategies
  | typeof ResourcePath.Versions
  | typeof ResourcePath.Experiments;

export interface ResourceDefinition {
  value: string;
  label: string;
  description?: string;
}

export interface ResourceFilter {
  strategy?: string;
  version?: string;
  backtest?: string;
  session_id?: string;
  experiment?: string;
}

/** Canonical query JSON: {"execution":{"id":"<uuid>"}}. Legacy capital ID is read, never written. */
export interface Execution {
  id: string;
}

export interface Metric {
  name: string;
}

export interface MyQuery extends DataQuery {
  queryType: QueryType;
  execution?: Execution;
  metrics?: Metric[];
  /** Resource a Query variable lists. Ignored by GetExecutionMetric. */
  source?: VariableSource;
  strategy?: string;
  version?: string;
  backtest?: string;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  queryType: QueryType.GetExecutionMetric,
  metrics: [{ name: 'portfolio_value' }],
};

export const DEFAULT_VARIABLE_QUERY: Partial<MyQuery> = {
  source: ResourcePath.Executions,
};

export interface MyDataSourceOptions extends DataSourceJsonData {
  path?: string;
}

export interface MySecureJsonData {
  apiKey?: string;
}
