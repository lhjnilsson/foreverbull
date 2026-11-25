import assert from "node:assert/strict";
import { test } from "node:test";
import { axisMoney, lastStoredPeriod, portfolioSeries } from "./equity.ts";
import type { ExecutionResult } from "./types.ts";

const day = (year: number, portfolioValue?: number, benchmarkPeriodReturn?: number): ExecutionResult => ({
  date: { year, month: 1, day: 2 },
  portfolioValue,
  benchmarkPeriodReturn,
});

test("an empty join is not a portfolio series", () => {
  const periods = [day(1, 0, 0)];
  assert.deepEqual(portfolioSeries(periods), []);
  assert.equal(lastStoredPeriod(periods), undefined);
});

test("stored days keep portfolio value and ignore benchmark return", () => {
  const periods = [day(2024, 100_000, 0), day(2024, 110_000, 0.04)];
  periods[1] = { ...periods[1], date: { year: 2024, month: 1, day: 3 } };
  assert.deepEqual(portfolioSeries(periods), [
    { date: "2024-01-02", portfolioValue: 100_000 },
    { date: "2024-01-03", portfolioValue: 110_000 },
  ]);
  assert.equal(lastStoredPeriod(periods)?.portfolioValue, 110_000);
});

test("a stored day without a finite portfolio value is omitted from the line", () => {
  const periods = [day(2024, Number.NaN), { date: { year: 2024, month: 2, day: 1 }, sharpe: 1.2 }];
  assert.deepEqual(portfolioSeries(periods), []);
  assert.equal(lastStoredPeriod(periods)?.sharpe, 1.2);
});

test("axis labels stay in currency units", () => {
  assert.equal(axisMoney(980), "980");
  assert.equal(axisMoney(12_500), "12.5k");
  assert.equal(axisMoney(2_000_000), "2m");
});
