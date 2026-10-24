import type { DateParts, ExecutionResult } from "./types";

export type EquityPoint = {
  date: string;
  portfolioValue: number;
};

// A year of 1 is the empty SQL join, not a stored session day.
export function hasStoredDate(period: ExecutionResult): boolean {
  const year = period.date?.year;
  const month = period.date?.month;
  const day = period.date?.day;
  return (
    typeof year === "number" &&
    year > 1 &&
    typeof month === "number" &&
    month >= 1 &&
    typeof day === "number" &&
    day >= 1
  );
}

export function portfolioSeries(periods: readonly ExecutionResult[] | undefined): EquityPoint[] {
  const points: EquityPoint[] = [];
  for (const period of periods ?? []) {
    if (!hasStoredDate(period)) continue;
    if (typeof period.portfolioValue !== "number" || !Number.isFinite(period.portfolioValue)) continue;
    points.push({ date: isoDate(period.date), portfolioValue: period.portfolioValue });
  }
  return points;
}

export function lastStoredPeriod(periods: readonly ExecutionResult[] | undefined): ExecutionResult | undefined {
  let last: ExecutionResult | undefined;
  for (const period of periods ?? []) {
    if (hasStoredDate(period)) last = period;
  }
  return last;
}

export function axisMoney(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `${compact(value / 1_000_000)}m`;
  if (abs >= 10_000) return `${compact(value / 1_000)}k`;
  return value.toLocaleString("en-US", { maximumFractionDigits: 0 });
}

function compact(value: number): string {
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

function isoDate(date: DateParts | undefined): string {
  const year = date?.year ?? 0;
  const month = date?.month ?? 0;
  const day = date?.day ?? 0;
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}
