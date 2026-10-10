import {
  DataSourceInstanceSettings,
  CoreApp,
  ScopedVars,
  CustomVariableSupport,
  DataQueryRequest,
  DataQueryResponse,
  DataFrame,
  arrayToDataFrame,
} from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv, locationService } from '@grafana/runtime';
import { Observable, from } from 'rxjs';
import { map, switchMap } from 'rxjs/operators';

import VariableQueryEditor from './components/VariableQueryEditor';
import { configurationFrame } from './configuration';
import {
  decideExecutionTimeRange,
  idsFromLocationSearch,
  idsFromVariableValues,
  resolveExecutionIds,
  spanFromExecutionOptions,
  spanFromFrames,
} from './executionRange';
import type { EpochRange, SpanSource } from './executionRange';
import {
  dashboardEntry,
  loadExecutionVariableOptions,
  shouldResetExecutionEntry,
  usesExecutionBacktestSelector,
} from './executionVariable';
import type { DashboardEntry, ExecutionPin, ExecutionVariableResult, ExecutionVariableState } from './executionVariable';
import {
  applyExecutionTemplates,
  isConfigurationQuery,
  isOverviewQuery,
  listVariableOptions,
  shouldForwardQuery,
} from './query';
import {
  DEFAULT_QUERY,
  DEFAULT_VARIABLE_QUERY,
  ExecutionArguments,
  ExecutionLabelStyle,
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
    this.datasource.syncExecutionEntry();
    if (usesExecutionBacktestSelector(query)) {
      const style: ExecutionLabelStyle = query.label === 'full' ? 'full' : 'dates';
      const loaded = await loadExecutionVariableOptions(
        this.datasource,
        this.datasource.executionVariableState(),
        replace(query.backtest),
        style
      );
      this.datasource.applyExecutionVariable(loaded);
      return loaded.options;
    }
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
  /** Page URL at first variable load, before Grafana writes var-backtest for a deep link. */
  private executionEntry: DashboardEntry | null = null;
  /** Backtest locked from a var-execution link that did not include var-backtest. */
  private executionPin: ExecutionPin | null = null;
  /** Execution key whose range we already wrote. A refresh must not snap a zoom back. */
  private appliedExecutionKey: string | null = null;
  /** Exact from/to last written for appliedExecutionKey. Series min/max replaces it when they differ. */
  private appliedRange: EpochRange | null = null;
  /** ListExecutions start/end, keyed by execution UUID, so the axis can move before frames return. */
  private executionSpans = new Map<string, { start: string; end: string }>();

  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
    this.variables = new DatasourceVariableSupport(this);
  }

  getDefaultQuery(_: CoreApp): Partial<MyQuery> {
    return DEFAULT_QUERY;
  }

  /**
   * Metric queries move the time axis. Configuration panels read /execution instead.
   * Overview queries use the dashboard time picker as a filter and must not move it.
   */
  query(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
    const targets = request.targets ?? [];
    const overviewTargets = targets.filter((target) => isOverviewQuery(target));
    const panelTargets = targets.filter((target) => !isOverviewQuery(target));
    if (overviewTargets.length === 0) {
      return this.queryExecutionPanels(request);
    }
    const overview = super.query({ ...request, targets: overviewTargets });
    if (panelTargets.length === 0) {
      return overview;
    }
    return overview.pipe(
      switchMap((overviewResponse) =>
        this.queryExecutionPanels({ ...request, targets: panelTargets }).pipe(
          map((panels) => ({
            ...panels,
            data: [...(overviewResponse.data ?? []), ...(panels.data ?? [])],
          }))
        )
      )
    );
  }

  private queryExecutionPanels(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
    const targets = request.targets ?? [];
    const configTargets = targets.filter((target) => isConfigurationQuery(target));
    if (configTargets.length === 0) {
      return this.queryMetrics(request);
    }
    const metricTargets = targets.filter((target) => !isConfigurationQuery(target));
    const config = from(this.loadConfigurationFrames(configTargets, request));
    if (metricTargets.length === 0) {
      return config.pipe(map((data) => ({ data })));
    }
    return config.pipe(
      switchMap((frames) =>
        this.queryMetrics({ ...request, targets: metricTargets }).pipe(
          map((response) => ({ ...response, data: [...frames, ...(response.data ?? [])] }))
        )
      )
    );
  }

  /**
   * ListExecutions start/end moves the axis as soon as the execution ids are known.
   * Period frames then replace that range when their min/max differ. No pad.
   * A deep link only needs var-execution; from/to are not required.
   */
  private queryMetrics(request: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
    const ids = resolveExecutionIds(
      (request.targets ?? []).map((target) => target.execution?.id),
      (value) => getTemplateSrv().replace(value, request.scopedVars),
      locationService.getSearchObject() as Record<string, unknown>
    );
    this.applyKnownSpan(ids, this.catalogSpan(ids), 'catalog', currentWindow(request));
    return super.query(request).pipe(
      map((response) => {
        this.applyKnownSpan(ids, spanFromFrames(responseFrames(response)), 'series', currentWindow(request));
        return response;
      })
    );
  }

  applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars) {
    return applyExecutionTemplates(query, (value) => getTemplateSrv().replace(value, scopedVars));
  }

  private applyKnownSpan(
    ids: Array<string | undefined | null>,
    span: EpochRange | null,
    spanSource: SpanSource,
    current: EpochRange | null
  ): void {
    const decision = decideExecutionTimeRange({
      executionIds: ids,
      span,
      spanSource,
      appliedExecutionKey: this.appliedExecutionKey,
      appliedRange: this.appliedRange,
      current,
    });
    if (!decision.apply || !decision.from || !decision.to) {
      return;
    }
    // Remember the id before touching the URL so the refresh this causes does not apply again.
    this.appliedExecutionKey = decision.executionKey;
    this.appliedRange = { fromMs: Number(decision.from), toMs: Number(decision.to) };
    locationService.partial({ from: decision.from, to: decision.to }, true);
  }

  private rememberExecutionSpans(items: ResourceDefinition[]): void {
    for (const item of items) {
      if (!item.value || !item.start || !item.end) {
        continue;
      }
      this.executionSpans.set(item.value, { start: item.start, end: item.end });
    }
  }

  private catalogSpan(ids: Array<string | undefined | null>): EpochRange | null {
    const options = [];
    for (const [id, span] of this.executionSpans) {
      options.push({ id, start: span.start, end: span.end });
    }
    return spanFromExecutionOptions(options, ids);
  }

  /** Dashboard variable, then the URL, so a deep link with no from/to still has an id. */
  private readSelectedExecutionIds(): string[] {
    try {
      const srv = getTemplateSrv();
      if (srv?.getVariables) {
        const fromVariables = idsFromVariableValues(
          srv.getVariables().map((variable) => ({
            name: variable.name,
            value: (variable as { current?: { value?: unknown } }).current?.value,
          }))
        );
        if (fromVariables.length > 0) {
          return fromVariables;
        }
      }
    } catch {
      // Template service is not ready outside a dashboard.
    }
    try {
      return idsFromLocationSearch(locationService.getSearchObject() as Record<string, unknown>);
    } catch {
      return [];
    }
  }

  /** Remember a ?var-execution= link before Grafana selects the first backtest name. */
  syncExecutionEntry(): void {
    let path = '';
    let search: Record<string, unknown> = {};
    try {
      path = locationService.getLocation()?.pathname ?? '';
      search = (locationService.getSearchObject() ?? {}) as Record<string, unknown>;
    } catch {
      return;
    }
    const next = dashboardEntry(path, search);
    if (shouldResetExecutionEntry(this.executionEntry, next)) {
      this.executionEntry = next;
      this.executionPin = null;
    }
  }

  executionVariableState(): ExecutionVariableState {
    return {
      entryExecutionId: this.executionEntry?.executionId ?? '',
      entryHadBacktest: this.executionEntry?.hadBacktest ?? false,
      pin: this.executionPin,
    };
  }

  applyExecutionVariable(loaded: ExecutionVariableResult): void {
    this.executionPin = loaded.state.pin;
    if (loaded.injected) {
      this.rememberExecutionSpans([loaded.injected]);
      const ids = this.readSelectedExecutionIds();
      this.applyKnownSpan(ids, this.catalogSpan(ids), 'catalog', null);
    }
    if (!loaded.setBacktest) {
      return;
    }
    try {
      locationService.partial({ 'var-backtest': loaded.setBacktest }, true);
    } catch {
      // Location service is not ready outside a dashboard.
    }
  }

  async getExecutions(filter?: ResourceFilter): Promise<ResourceDefinition[]> {
    const items = await this.postResource<ResourceDefinition[]>(
      ResourcePath.Executions,
      interpolateFilter(filter) ?? {}
    );
    this.rememberExecutionSpans(items);
    const ids = this.readSelectedExecutionIds();
    this.applyKnownSpan(ids, this.catalogSpan(ids), 'catalog', null);
    return items;
  }

  async getBacktests(): Promise<ResourceDefinition[]> {
    return this.postResource<ResourceDefinition[]>(ResourcePath.Backtests, {});
  }

  async getExecutionArguments(id: string): Promise<ExecutionArguments | null> {
    if (!id) {
      return null;
    }
    const view = await this.postResource<ExecutionArguments>(ResourcePath.Execution, { id });
    if (!view?.id) {
      return null;
    }
    return { ...view, functions: view.functions ?? [] };
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
    return shouldForwardQuery(query);
  }

  private async loadConfigurationFrames(
    targets: MyQuery[],
    request: DataQueryRequest<MyQuery>
  ): Promise<DataFrame[]> {
    const frames: DataFrame[] = [];
    for (const target of targets) {
      const raw = target.execution?.id ?? '';
      const replaced = raw ? getTemplateSrv().replace(raw, request.scopedVars) : '';
      const id = replaced.includes('$') ? '' : replaced.trim();
      let view: ExecutionArguments | null = null;
      if (id) {
        try {
          view = await this.getExecutionArguments(id);
        } catch {
          view = null;
        }
      }
      frames.push(configurationFrame(target.configuration, target.refId, view));
    }
    return frames;
  }
}

function responseFrames(response: DataQueryResponse): DataFrame[] {
  const frames: DataFrame[] = [];
  for (const item of response.data ?? []) {
    if (Array.isArray(item)) {
      frames.push(...item);
    } else if (item && typeof item === 'object' && 'fields' in item) {
      frames.push(item);
    }
  }
  return frames;
}

function currentWindow(request: DataQueryRequest<MyQuery>): { fromMs: number; toMs: number } | null {
  const fromMs = request.range?.from?.valueOf?.();
  const toMs = request.range?.to?.valueOf?.();
  if (typeof fromMs !== 'number' || typeof toMs !== 'number') {
    return null;
  }
  if (!Number.isFinite(fromMs) || !Number.isFinite(toMs)) {
    return null;
  }
  return { fromMs, toMs };
}
