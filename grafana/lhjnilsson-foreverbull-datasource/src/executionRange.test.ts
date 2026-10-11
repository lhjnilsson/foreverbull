import {
  decideExecutionTimeRange,
  rangeIsInside,
  resolveExecutionIds,
  spanFromExecutionOptions,
  spanFromFrames,
} from './executionRange';

const RUN_2025 = {
  fromMs: Date.UTC(2025, 0, 1),
  toMs: Date.UTC(2025, 11, 31),
};

const RUN_NEXT = {
  fromMs: Date.UTC(2020, 0, 1),
  toMs: Date.UTC(2024, 5, 15),
};

/** Resolved now-30y as of 2026-10-03. The axis must not stay here. */
const NOW_30Y = {
  fromMs: Date.UTC(1996, 9, 3),
  toMs: Date.UTC(2026, 9, 3),
};

/** Saved dashboard time.from/time.to of now/now, resolved on 2026-10-05. */
const NOW_NOW = {
  fromMs: Date.UTC(2026, 9, 5, 15, 0, 0),
  toMs: Date.UTC(2026, 9, 5, 15, 0, 0),
};

function yearOf(epochMs: string | undefined): number {
  return new Date(Number(epochMs)).getUTCFullYear();
}

describe('execution time axis', () => {
  it('binds 2025-01-01 through 2025-12-31 to those exact timestamps', () => {
    expect(yearOf(String(NOW_30Y.fromMs))).toBe(1996);

    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: RUN_2025,
      spanSource: 'catalog',
      appliedExecutionKey: null,
      current: NOW_30Y,
    });

    expect(decision.apply).toBe(true);
    expect(decision.from).toBe(String(RUN_2025.fromMs));
    expect(decision.to).toBe(String(RUN_2025.toMs));
    expect(decision.from).toBe(String(Date.UTC(2025, 0, 1)));
    expect(decision.to).toBe(String(Date.UTC(2025, 11, 31)));
    expect(decision.from).not.toBe('now');
    expect(decision.to).not.toBe('now');
    expect(decision.from).not.toBe('now-30y');
    expect(decision.to).not.toBe('now-30y');
    expect(yearOf(decision.from)).not.toBe(1996);
    expect(yearOf(decision.from)).not.toBe(1997);
    expect(yearOf(decision.to)).not.toBe(1996);
    expect(yearOf(decision.to)).not.toBe(1997);
    expect(Number(decision.from)).toBe(RUN_2025.fromMs);
    expect(Number(decision.to)).toBe(RUN_2025.toMs);
  });

  it('applies on first entry when the window is now/now and does not contain the points', () => {
    expect(rangeIsInside(NOW_NOW, RUN_2025)).toBe(false);

    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: RUN_2025,
      spanSource: 'catalog',
      appliedExecutionKey: null,
      current: NOW_NOW,
    });

    expect(decision.apply).toBe(true);
    expect(decision.executionKey).toBe('exec-2025');
    expect(decision.from).toBe(String(Date.UTC(2025, 0, 1)));
    expect(decision.to).toBe(String(Date.UTC(2025, 11, 31)));
    expect(decision.from).not.toBe('now');
    expect(decision.to).not.toBe('now');
  });

  it('replaces both ends when a second execution id is selected, even if the zoom sits inside', () => {
    const first = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: RUN_2025,
      appliedExecutionKey: null,
      current: NOW_NOW,
    });
    const zoom = { fromMs: Date.UTC(2022, 2, 1), toMs: Date.UTC(2023, 5, 1) };

    expect(rangeIsInside(zoom, RUN_NEXT)).toBe(true);

    const second = decideExecutionTimeRange({
      executionIds: ['exec-next'],
      span: RUN_NEXT,
      spanSource: 'catalog',
      appliedExecutionKey: first.executionKey,
      appliedRange: RUN_2025,
      current: zoom,
    });

    expect(second.apply).toBe(true);
    expect(second.executionKey).toBe('exec-next');
    expect(second.executionKey).not.toBe(first.executionKey);
    expect(second.from).toBe(String(RUN_NEXT.fromMs));
    expect(second.to).toBe(String(RUN_NEXT.toMs));
    expect(Number(second.from)).toBe(Date.UTC(2020, 0, 1));
    expect(Number(second.to)).toBe(Date.UTC(2024, 5, 15));
    expect(Number(second.from)).not.toBe(zoom.fromMs);
    expect(Number(second.to)).not.toBe(zoom.toMs);
    expect(second.from).not.toBe('now');
    expect(second.from).not.toBe('now-30y');
    expect(yearOf(second.from)).not.toBe(1996);
    expect(yearOf(second.from)).not.toBe(1997);
  });

  it('replaces Last 30 years for the same execution when that window is still showing', () => {
    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: RUN_2025,
      spanSource: 'series',
      appliedExecutionKey: 'exec-2025',
      appliedRange: RUN_2025,
      current: NOW_30Y,
    });

    expect(rangeIsInside(NOW_30Y, RUN_2025)).toBe(false);
    expect(decision.apply).toBe(true);
    expect(decision.from).toBe(String(RUN_2025.fromMs));
    expect(decision.to).toBe(String(RUN_2025.toMs));
    expect(decision.from).not.toBe('now-30y');
    expect(yearOf(decision.from)).toBe(2025);
  });

  it('reads the execution id out of the $execution placeholder', () => {
    expect(resolveExecutionIds(['$execution'], (value) => value.replace('$execution', 'exec-2025'))).toEqual([
      'exec-2025',
    ]);
    expect(
      resolveExecutionIds(['$execution'], (value) => value, { 'var-execution': 'exec-from-url' })
    ).toEqual(['exec-from-url']);
  });

  it('keeps a later zoom after that min and max was applied for the same id', () => {
    const zoom = { fromMs: Date.UTC(2025, 2, 1), toMs: Date.UTC(2025, 5, 1) };
    expect(rangeIsInside(zoom, RUN_2025)).toBe(true);

    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: RUN_2025,
      spanSource: 'series',
      appliedExecutionKey: 'exec-2025',
      appliedRange: RUN_2025,
      current: zoom,
    });

    expect(decision.apply).toBe(false);
    expect(decision.executionKey).toBe('exec-2025');
    expect(decision.from).toBeUndefined();
    expect(decision.to).toBeUndefined();
  });

  it('replaces ListExecutions dates with series min and max when they differ', () => {
    const series = { fromMs: Date.UTC(2025, 0, 2), toMs: Date.UTC(2025, 11, 30) };
    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: series,
      spanSource: 'series',
      appliedExecutionKey: 'exec-2025',
      appliedRange: RUN_2025,
      current: RUN_2025,
    });

    expect(decision.apply).toBe(true);
    expect(decision.from).toBe(String(series.fromMs));
    expect(decision.to).toBe(String(series.toMs));
  });

  it('does not let catalog dates snap a zoom back after series min and max was applied', () => {
    const series = { fromMs: Date.UTC(2025, 0, 2), toMs: Date.UTC(2025, 11, 30) };
    const zoom = { fromMs: Date.UTC(2025, 3, 1), toMs: Date.UTC(2025, 4, 1) };
    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025'],
      span: RUN_2025,
      spanSource: 'catalog',
      appliedExecutionKey: 'exec-2025',
      appliedRange: series,
      current: zoom,
    });

    expect(decision.apply).toBe(false);
    expect(decision.from).toBeUndefined();
    expect(decision.to).toBeUndefined();
  });

  it('uses the union of overlaid runs so neither sits on a 30-year axis', () => {
    const span = spanFromFrames([
      { fields: [{ name: 'time', type: 'time', values: [Date.UTC(2020, 0, 1), Date.UTC(2020, 5, 1)] }] },
      { fields: [{ name: 'time', type: 'time', values: [Date.UTC(2025, 0, 1), Date.UTC(2025, 11, 31)] }] },
    ]);
    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025', 'exec-2020'],
      span,
      spanSource: 'series',
      appliedExecutionKey: null,
      current: NOW_30Y,
    });

    expect(decision.apply).toBe(true);
    expect(decision.executionKey).toBe('exec-2020|exec-2025');
    expect(decision.from).toBe(String(Date.UTC(2020, 0, 1)));
    expect(decision.to).toBe(String(Date.UTC(2025, 11, 31)));
    expect(yearOf(decision.from)).not.toBe(1996);
    expect(yearOf(decision.from)).not.toBe(1997);
  });

  it('uses the union of selected execution start and end with no pad', () => {
    const span = spanFromExecutionOptions(
      [
        { id: 'exec-2020', start: '2020-01-01', end: '2020-06-01' },
        { id: 'exec-2025', start: '2025-01-01', end: '2025-12-31' },
        { id: 'exec-other', start: '1996-01-01', end: '1997-01-01' },
      ],
      ['exec-2025', 'exec-2020']
    );
    const decision = decideExecutionTimeRange({
      executionIds: ['exec-2025', 'exec-2020'],
      span,
      spanSource: 'catalog',
      appliedExecutionKey: null,
      current: NOW_NOW,
    });

    expect(span).toEqual({ fromMs: Date.UTC(2020, 0, 1), toMs: Date.UTC(2025, 11, 31) });
    expect(decision.apply).toBe(true);
    expect(decision.executionKey).toBe('exec-2020|exec-2025');
    expect(decision.from).toBe(String(Date.UTC(2020, 0, 1)));
    expect(decision.to).toBe(String(Date.UTC(2025, 11, 31)));
    expect(yearOf(decision.from)).not.toBe(1996);
    expect(yearOf(decision.from)).not.toBe(1997);
  });
});
