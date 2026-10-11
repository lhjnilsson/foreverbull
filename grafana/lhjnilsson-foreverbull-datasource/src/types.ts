import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export enum QueryType {
  GetExecutionMetric = 'GetExecutionMetric',
  /** Table of executions overlapping the dashboard time picker. Not a metric series. */
  ListExecutionOverview = 'ListExecutionOverview',
}

/** Backend resource routes. Execution and experiment values are execution UUIDs. */
export const ResourcePath = {
  Executions: 'executions',
  Execution: 'execution',
  Backtests: 'backtests',
  Metrics: 'metrics',
  Strategies: 'strategies',
  Versions: 'versions',
  Experiments: 'experiments',
} as const;

export type VariableSource =
  | typeof ResourcePath.Executions
  | typeof ResourcePath.Backtests
  | typeof ResourcePath.Strategies
  | typeof ResourcePath.Versions
  | typeof ResourcePath.Experiments;

/** Execution-dashboard picker text. `dates` is start–end and a short id. */
export type ExecutionLabelStyle = 'full' | 'dates';

/** Panel target that reads /execution instead of plotting GetExecutionMetric. */
export type ConfigurationView = 'metadata' | 'arguments';

export interface ExecutionParameter {
  key: string;
  value: string;
  value_type: string;
}

export interface ExecutionFunction {
  name: string;
  parameters?: ExecutionParameter[];
}

/** One run from GET/POST /execution. No database URL or ports. */
export interface ExecutionArguments {
  id: string;
  session: string;
  backtest: string;
  benchmark?: string;
  start: string;
  end: string;
  functions?: ExecutionFunction[];
}

export interface ResourceDefinition {
  value: string;
  label: string;
  description?: string;
  /** YYYY-MM-DD from ListExecutions start_date. Execution options only. */
  start?: string;
  /** YYYY-MM-DD from ListExecutions end_date. Execution options only. */
  end?: string;
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
  /** Execution variable text. `dates` drops the backtest name (it has its own selector). */
  label?: ExecutionLabelStyle;
  /** When set, the panel lists stored run metadata or worker arguments. Not a metric series. */
  configuration?: ConfigurationView;
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
