import {
  DataSourceInstanceSettings,
  CoreApp,
  ScopedVars,
  CustomVariableSupport,
  DataQueryRequest,
  DataQueryResponse,
  arrayToDataFrame,
} from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import { Observable, from } from 'rxjs';
import { map } from 'rxjs/operators';

import VariableQueryEditor from './components/VariableQueryEditor';
import {
  applyExecutionTemplates,
  isRunnableQuery,
  listVariableOptions,
} from './query';
import {
  DEFAULT_QUERY,
  DEFAULT_VARIABLE_QUERY,
  MyQuery,
  MyDataSourceOptions,
  ResourceDefinition,
  ResourceFilter,
  ResourcePath,
} from './types';

function interpolateFilter(filter?: ResourceFilter): ResourceFilter | undefined {
  if (!filter) {
    return undefined;
  }
  const replace = (value?: string) => (value ? getTemplateSrv().replace(value) : '');
  const interpolated: ResourceFilter = {
    strategy: replace(filter.strategy),
    version: replace(filter.version),
    backtest: replace(filter.backtest),
    session_id: replace(filter.session_id),
    experiment: replace(filter.experiment),
  };
  const hasValue = Object.values(interpolated).some((value) => Boolean(value));
  return hasValue ? interpolated : undefined;
}

export class DatasourceVariableSupport extends CustomVariableSupport<DataSource, MyQuery> {
  editor = VariableQueryEditor;

  constructor(private datasource: DataSource) {
    super();
    this.query = this.query.bind(this);
  }

  getDefaultQuery(): Partial<MyQuery> {
    return DEFAULT_VARIABLE_QUERY;
  }

  async execute(query: MyQuery) {
    const replace = (value?: string) => (value ? getTemplateSrv().replace(value) : value);
    return listVariableOptions(this.datasource, {
      ...query,
      strategy: replace(query.strategy),
      version: replace(query.version),
      backtest: replace(query.backtest),
    });
  }

  query(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
    const result = this.execute(request.targets[0]);
    return from(result).pipe(map((data) => ({ data: [arrayToDataFrame(data)] })));
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
    return applyExecutionTemplates(query, (value) => getTemplateSrv().replace(value, scopedVars));
  }

  async getExecutions(filter?: ResourceFilter): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>(ResourcePath.Executions, interpolateFilter(filter) ?? {});
  }

  async getMetrics(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>(ResourcePath.Metrics);
  }

  async getStrategies(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>(ResourcePath.Strategies);
  }

  async getVersions(filter?: ResourceFilter): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>(ResourcePath.Versions, interpolateFilter(filter) ?? {});
  }

  async getExperiments(filter?: ResourceFilter): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>(ResourcePath.Experiments, interpolateFilter(filter) ?? {});
  }

  filterQuery(query: MyQuery): boolean {
    return isRunnableQuery(query);
  }
}
