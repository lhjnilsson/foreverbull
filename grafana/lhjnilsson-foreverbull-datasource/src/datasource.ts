import {
  DataSourceInstanceSettings,
  CoreApp,
  ScopedVars,
  SelectableValue,
  CustomVariableSupport,
  DataQueryRequest,
  DataQueryResponse,
} from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { MyQuery, MyDataSourceOptions, DEFAULT_QUERY } from './types';

export interface ResourceDefinition {
  value?: string;
  label?: string;
  description?: string;
}

import VariableQueryEditor from './components/VariableQueryEditor';

import { Observable, from } from 'rxjs';
import { map } from 'rxjs/operators';

export class DatasourceVariableSupport extends CustomVariableSupport<DataSource> {
  editor = VariableQueryEditor;

  constructor(private datasource: DataSource) {
    super();
    this.query = this.query.bind(this);
  }

  async execute(query: MyQuery) {
    return this.datasource.getExecutions();
  }

  query(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
    const result = this.execute(request.targets[0]);

    return from(result).pipe(map((data) => ({ data })));
  }
}

export class DataSource extends DataSourceWithBackend<MyQuery, MyDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
    this.variables = new DatasourceVariableSupport(this);
  }

  getDefaultQuery(_: CoreApp): Partial<MyQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars) {
    return {
      ...query,
      execution: { ID: getTemplateSrv().replace(query.execution?.ID, scopedVars) },
    };
  }

  async getExecutions(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>('executions');
  }

  async getMetrics(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>('metrics');
  }

  filterQuery(query: MyQuery): boolean {
    return !!query.execution;
  }
}
