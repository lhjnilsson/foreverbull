import assert from "node:assert/strict";
import { test } from "node:test";
import {
  DIVIDEND_LIST_UNREADABLE,
  DIVIDENDS_LOAD_ERROR,
  NO_DIVIDENDS,
  PAY_DATE_UNKNOWN,
  dividendLines,
  payDateText,
  perShare,
  type StoredDividend,
} from "./dividends.ts";

const ko: StoredDividend = {
  symbol: "KO",
  ex_date: "2024-03-14",
  amount: 0.485,
  pay_date: "2024-04-01",
};

test("the list shows the symbol, the ex-date, the per-share amount and the pay date", () => {
  assert.equal(perShare(0.24), "$0.24 per share");
  assert.equal(perShare(0.485), "$0.485 per share");
  assert.deepEqual(dividendLines([ko, { symbol: "TSM", ex_date: "2024-06-13", amount: 0.5, pay_date: null }]), [
    { symbol: "KO", exDate: "2024-03-14", amount: "$0.485 per share", payDate: "2024-04-01" },
    { symbol: "TSM", exDate: "2024-06-13", amount: "$0.50 per share", payDate: PAY_DATE_UNKNOWN },
  ]);
});

test("a missing pay date is pay date unknown and is not estimated", () => {
  assert.equal(PAY_DATE_UNKNOWN, "pay date unknown");
  assert.equal(payDateText(null), "pay date unknown");
  assert.equal(payDateText(undefined), "pay date unknown");
  assert.equal(payDateText(""), "pay date unknown");
  assert.equal(payDateText("   "), "pay date unknown");
  assert.equal(payDateText("2024-04-01"), "2024-04-01");
  assert.equal(dividendLines([{ ...ko, pay_date: null }])[0]?.payDate, "pay date unknown");
});

test("no stored dividends is the empty state", () => {
  assert.equal(NO_DIVIDENDS, "No dividends stored.");
  assert.deepEqual(dividendLines([]), []);
});

test("a failed load and an unreadable list use the report sentences", () => {
  assert.equal(
    DIVIDENDS_LOAD_ERROR,
    "The stored dividends couldn't be loaded. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
  );
  assert.equal(
    DIVIDEND_LIST_UNREADABLE,
    "The dividend list couldn't be read. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
  );
});
