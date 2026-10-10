import {
  applyExecutionTemplates,
  executionRefId,
  isOverviewQuery,
  isRunnableQuery,
  listVariableOptions,
  shouldForwardQuery,
  toExecution,
  toMetrics,
  variableDefinition,
  variableSource,
  withExecutionMetric,
} from './query';
import { QueryType, ResourcePath } from './types';

describe('execution query JSON', () => {
  it('reads canonical id and legacy ID', () => {
    expect(executionRefId({ id: 'abc' })).toBe('abc');
    expect(executionRefId({ ID: 'legacy' })).toBe('legacy');
    expect(executionRefId({ id: 'canonical', ID: 'legacy' })).toBe('canonical');
    expect(executionRefId(undefined)).toBe('');
  });

  it('writes execution.id objects, never executionId', () => {
    const query = withExecutionMetric(
      { refId: 'A', queryType: QueryType.GetExecutionMetric },
      { execution: toExecution('run-1'), metrics: toMetrics(['portfolio_value', 'sharpe']) }
    );

    expect(query).toEqual({
      refId: 'A',
      queryType: QueryType.GetExecutionMetric,
      execution: { id: 'run-1' },
      metrics: [{ name: 'portfolio_value' }, { name: 'sharpe' }],
    });
    expect(query).not.toHaveProperty('executionId');
    expect(JSON.stringify(query)).toContain('"id":"run-1"');
    expect(JSON.stringify(query)).not.toContain('executionId');
    expect(JSON.stringify(query)).not.toContain('"ID"');
  });

  it('interpolates $execution onto execution.id', () => {
    const query = applyExecutionTemplates(
      { refId: 'A', queryType: QueryType.GetExecutionMetric, execution: { id: '$execution' } },
      (value) => value.replace('$execution', 'uuid-1')
    );

    expect(query.execution).toEqual({ id: 'uuid-1' });
    expect(query).not.toHaveProperty('executionId');
  });

  it('requires execution and metrics before running', () => {
    expect(isRunnableQuery({ refId: 'A', queryType: QueryType.GetExecutionMetric })).toBe(false);
    expect(
      isRunnableQuery({
        refId: 'A',
        queryType: QueryType.GetExecutionMetric,
        execution: { id: 'uuid-1' },
        metrics: [{ name: 'portfolio_value' }],
      })
    ).toBe(true);
  });

  it('keeps ListExecutionOverview and interpolates the backtest filter', () => {
    const query = applyExecutionTemplates(
      { refId: 'A', queryType: QueryType.ListExecutionOverview, backtest: '$backtest' },
      (value) => (value === '$backtest' ? '*' : value)
    );

    expect(query.queryType).toBe(QueryType.ListExecutionOverview);
    expect(query.backtest).toBe('*');
    expect(isOverviewQuery(query)).toBe(true);
    expect(shouldForwardQuery(query)).toBe(true);
    expect(query).not.toHaveProperty('executionId');
    expect(JSON.stringify(query)).not.toContain('executionId');
  });

  it('does not require an execution id for the overview table', () => {
    expect(shouldForwardQuery({ refId: 'A', queryType: QueryType.ListExecutionOverview })).toBe(true);
    expect(shouldForwardQuery({ refId: 'A', queryType: QueryType.GetExecutionMetric })).toBe(false);
  });
});

describe('variable queries', () => {
  const ds = {
    getExecutions: jest.fn().mockResolvedValue([{ value: 'exec-1', label: 'spy 2020-01-01–2020-12-31 exec-1' }]),
    getBacktests: jest.fn().mockResolvedValue([
      { value: 'spy', label: 'spy' },
      { value: 'eu-small', label: 'eu-small' },
    ]),
    getStrategies: jest.fn().mockResolvedValue([{ value: 'mean-rev', label: 'mean-rev' }]),
    getVersions: jest.fn().mockResolvedValue([{ value: 'ver-1', label: 'rev 1' }]),
    getExperiments: jest.fn().mockResolvedValue([{ value: 'exec-1', label: 'mean-rev / rev 1 / spy' }]),
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('defaults to /executions so $execution is populated', async () => {
    expect(variableSource({})).toBe(ResourcePath.Executions);
    expect(variableDefinition({ refId: 'A', queryType: QueryType.GetExecutionMetric })).toBe('executions');

    const values = await listVariableOptions(ds, { refId: 'A', queryType: QueryType.GetExecutionMetric });
    expect(ds.getExecutions).toHaveBeenCalledWith({ strategy: undefined, version: undefined, backtest: undefined });
    expect(values).toEqual([{ text: 'spy 2020-01-01–2020-12-31 exec-1', value: 'exec-1' }]);
  });

  it('lists strategies and experiments from their resource routes', async () => {
    await listVariableOptions(ds, {
      refId: 'A',
      queryType: QueryType.GetExecutionMetric,
      source: ResourcePath.Strategies,
    });
    expect(ds.getStrategies).toHaveBeenCalled();

    await listVariableOptions(ds, {
      refId: 'A',
      queryType: QueryType.GetExecutionMetric,
      source: ResourcePath.Experiments,
      strategy: 'mean-rev',
      version: 'ver-1',
    });
    expect(ds.getExperiments).toHaveBeenCalledWith({
      strategy: 'mean-rev',
      version: 'ver-1',
      backtest: undefined,
    });
  });

  it('lists backtest names as value and label', async () => {
    const values = await listVariableOptions(ds, {
      refId: 'A',
      queryType: QueryType.GetExecutionMetric,
      source: ResourcePath.Backtests,
    });
    expect(ds.getBacktests).toHaveBeenCalled();
    expect(values).toEqual([
      { text: 'spy', value: 'spy' },
      { text: 'eu-small', value: 'eu-small' },
    ]);
    expect(JSON.stringify(values)).not.toContain('executionId');
  });
});
