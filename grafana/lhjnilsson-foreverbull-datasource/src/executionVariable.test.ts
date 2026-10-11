import { loadExecutionVariableOptions, type ExecutionVariableClient, type ExecutionVariableState } from './executionVariable';
import type { ExecutionArguments, ResourceDefinition, ResourceFilter } from './types';

const SPY_ID = '84753648-1bba-4a76-8c5f-c0631204633e';
const EU_ID = '11111111-2222-3333-4444-555555555555';

const spyRun: ResourceDefinition = {
  value: SPY_ID,
  label: 'nasdaq 2023-01-03–2023-12-25 84753648',
  start: '2023-01-03',
  end: '2023-12-25',
};

const euRun: ResourceDefinition = {
  value: EU_ID,
  label: 'eu 2022-01-01–2022-06-01 11111111',
  start: '2022-01-01',
  end: '2022-06-01',
};

const spyArguments: ExecutionArguments = {
  id: SPY_ID,
  session: 'sess-1',
  backtest: 'nasdaq',
  benchmark: 'QQQ',
  start: '2023-01-03',
  end: '2023-12-25',
  functions: [],
};

function client(lists: Record<string, ResourceDefinition[]>): ExecutionVariableClient & {
  getExecutions: jest.Mock;
  getExecutionArguments: jest.Mock;
} {
  return {
    getExecutions: jest.fn(async (filter?: ResourceFilter) => lists[filter?.backtest ?? ''] ?? []),
    getExecutionArguments: jest.fn(async (id: string) => (id === SPY_ID ? spyArguments : null)),
  };
}

describe('execution variable', () => {
  it('resolves a var-execution deep link onto that run’s backtest', async () => {
    const ds = client({ nasdaq: [spyRun], eu: [euRun] });
    const state: ExecutionVariableState = {
      entryExecutionId: SPY_ID,
      entryHadBacktest: false,
      pin: null,
    };

    const loaded = await loadExecutionVariableOptions(ds, state, 'eu', 'dates');

    expect(ds.getExecutionArguments).toHaveBeenCalledWith(SPY_ID);
    expect(ds.getExecutions).toHaveBeenCalledWith({ backtest: 'nasdaq' });
    expect(loaded.setBacktest).toBe('nasdaq');
    expect(loaded.options.map((option) => option.value)).toEqual([SPY_ID]);
    expect(loaded.options[0].text).toBe('2023-01-03–2023-12-25 84753648');
    expect(String(loaded.options[0].text)).not.toContain('nasdaq');
    expect(loaded.options.map((option) => option.value)).not.toContain(EU_ID);
  });

  it('keeps the deep-linked execution when that backtest’s list misses it', async () => {
    const ds = client({ nasdaq: [], eu: [euRun] });
    const loaded = await loadExecutionVariableOptions(
      ds,
      { entryExecutionId: SPY_ID, entryHadBacktest: false, pin: null },
      '',
      'dates'
    );

    expect(ds.getExecutions).toHaveBeenCalledWith({ backtest: 'nasdaq' });
    expect(loaded.setBacktest).toBe('nasdaq');
    expect(loaded.options).toEqual([{ text: '2023-01-03–2023-12-25 84753648', value: SPY_ID }]);
  });

  it('limits execution options to the selected backtest', async () => {
    const ds = client({ nasdaq: [spyRun], eu: [euRun] });
    const state: ExecutionVariableState = {
      entryExecutionId: SPY_ID,
      entryHadBacktest: false,
      pin: { backtest: 'nasdaq', acknowledged: true },
    };

    const loaded = await loadExecutionVariableOptions(ds, state, 'eu', 'dates');

    expect(ds.getExecutionArguments).not.toHaveBeenCalled();
    expect(ds.getExecutions).toHaveBeenCalledWith({ backtest: 'eu' });
    expect(loaded.setBacktest).toBeUndefined();
    expect(loaded.options.map((option) => option.value)).toEqual([EU_ID]);
    expect(loaded.options.map((option) => option.value)).not.toContain(SPY_ID);
  });
});
