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

export interface MyQuery extends DataQuery {
  queryType: QueryType;
  executionId?: string;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  queryType: QueryType.GetExecutionMetric,
};

export interface MyDataSourceOptions extends DataSourceJsonData {}
