import { DataSourceInstanceSettings, CoreApp, ScopedVars, SelectableValue } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { MyQuery, MyDataSourceOptions, DEFAULT_QUERY } from './types';

export interface ResourceDefinition {
  value?: string;
  label?: string;
  description?: string;
}

export class DataSource extends DataSourceWithBackend<MyQuery, MyDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<MyQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars) {
    return {
      ...query,
      queryText: getTemplateSrv().replace(query.queryType, scopedVars),
    };
  }

  async getExecutions(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>('executions');
  }

  async getMetrics(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>('metrics');
  }

  filterQuery(query: MyQuery): boolean {
    if (!!query.QueryType) {
      return false;
    }
    return true;
  }
}
