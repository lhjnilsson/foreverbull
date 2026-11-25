// A stored dividend of the backtest, as GET /api/backtests/{name}/dividends
// sends it. pay_date is null when the provider sent none. It is never filled in.
export type StoredDividend = {
  symbol: string;
  ex_date: string;
  amount: number;
  pay_date: string | null;
};

export type DividendLine = {
  symbol: string;
  exDate: string;
  amount: string;
  payDate: string;
};

export const PAY_DATE_UNKNOWN = "pay date unknown";

export const NO_DIVIDENDS = "No dividends stored.";

export const DIVIDENDS_LOAD_ERROR =
  "The stored dividends couldn't be loaded. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.";

export const DIVIDEND_LIST_UNREADABLE =
  "The dividend list couldn't be read. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.";

// The per-share amount the provider stored. At least two decimals, and no
// rounding away a rate such as 0.485.
export function perShare(amount: number): string {
  const text = Math.abs(amount).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 6 });
  if (amount < 0) return `-$${text} per share`;
  return `$${text} per share`;
}

// A missing pay date stays missing. Nothing estimates one.
export function payDateText(payDate: string | null | undefined): string {
  if (typeof payDate !== "string" || payDate.trim() === "") return PAY_DATE_UNKNOWN;
  return payDate;
}

export function dividendLines(rows: readonly StoredDividend[]): DividendLine[] {
  return rows.map((row) => ({
    symbol: row.symbol.trim(),
    exDate: row.ex_date.trim() === "" ? "—" : row.ex_date,
    amount: perShare(row.amount),
    payDate: payDateText(row.pay_date),
  }));
}
