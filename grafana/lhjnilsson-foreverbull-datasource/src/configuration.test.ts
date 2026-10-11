import { argumentsFrame, frameText, metadataFrame } from './configuration';
import type { ExecutionArguments } from './types';

const stored: ExecutionArguments = {
  id: '84753648-1bba-4a76-8c5f-c0631204633e',
  session: 'sess-1',
  backtest: 'nasdaq',
  benchmark: 'QQQ',
  start: '2023-01-03',
  end: '2023-12-25',
  functions: [
    {
      name: 'rebalance',
      parameters: [{ key: 'fastperiod', value: '12', value_type: 'int' }],
    },
  ],
};

const legacy: ExecutionArguments = {
  id: 'legacy-exec',
  session: 'sess-old',
  backtest: 'nasdaq',
  benchmark: 'QQQ',
  start: '2020-01-01',
  end: '2020-12-31',
  functions: [],
};

describe('execution configuration frames', () => {
  it('shows stored function name, key, value, and value type', () => {
    const frame = argumentsFrame('A', stored);
    expect(frame.length).toBe(1);
    expect(frame.fields.map((field) => field.name)).toEqual(['function', 'key', 'value', 'value_type']);
    expect(frameText(frame)).toContain('rebalance');
    expect(frameText(frame)).toContain('fastperiod');
    expect(frameText(frame)).toContain('12');
    expect(frameText(frame)).toContain('int');
  });

  it('shows an empty configuration when nothing was stored', () => {
    const frame = argumentsFrame('A', legacy);
    expect(frame.length).toBe(0);
    const text = frameText(frame);
    expect(text).not.toMatch(/fastperiod|slowperiod|signalperiod|max_positions|cash_reserve|timeperiod|oversold/);
    expect(text).not.toMatch(/database|broker|namespace/i);
  });

  it('shows backtest, benchmark, session, dates, and execution id', () => {
    const frame = metadataFrame('A', stored);
    expect(frame.length).toBe(1);
    expect(frame.fields.map((field) => field.name)).toEqual([
      'backtest',
      'benchmark',
      'session',
      'start',
      'end',
      'execution',
    ]);
    const text = frameText(frame);
    expect(text).toContain('nasdaq');
    expect(text).toContain('QQQ');
    expect(text).toContain('sess-1');
    expect(text).toContain('2023-01-03');
    expect(text).toContain('2023-12-25');
    expect(text).toContain(stored.id);
    expect(text).not.toMatch(/database|broker_port|namespace_port|namespace port/i);
  });
});
