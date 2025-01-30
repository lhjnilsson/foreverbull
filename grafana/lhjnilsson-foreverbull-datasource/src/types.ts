import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export enum QueryType {
  GetExecutionMetric = 'GetExecutionMetric',
}

export interface ResourceDefinition {
  value: string;
  label: string;
  description: string;
}

export interface Execution {
  ID: string;
}

export interface Metric {
  name: string;
}

export interface MyQuery extends DataQuery {
  queryType: QueryType;
  execution?: Execution;
  metrics?: Metric[];
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  queryType: QueryType.GetExecutionMetric,
};

export interface MyDataSourceOptions extends DataSourceJsonData {}
