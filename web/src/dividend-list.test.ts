import assert from "node:assert/strict";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { DividendList } from "./dividend-list.ts";
import { DIVIDEND_LIST_UNREADABLE, DIVIDENDS_LOAD_ERROR, NO_DIVIDENDS } from "./dividends.ts";
import type { StoredDividend } from "./dividends.ts";
import type { Load } from "./useJson.ts";

function html(load: Load<StoredDividend[]>) {
  return renderToStaticMarkup(createElement(DividendList, { load }));
}

// renderToStaticMarkup escapes the apostrophe. The page shows the sentence.
function text(markup: string) {
  return markup.replace(/<[^>]*>/g, "").replaceAll("&#x27;", "'").replaceAll("&amp;", "&");
}

const ko: StoredDividend = {
  symbol: "KO",
  ex_date: "2024-03-14",
  amount: 0.485,
  pay_date: "2024-04-01",
};

test("loading shows the stored-dividends line and not an error", () => {
  const rendered = html({ state: "loading" });
  assert.match(rendered, /Loading the stored dividends\./);
  assert.equal(rendered.includes(DIVIDENDS_LOAD_ERROR), false);
  assert.equal(rendered.includes(NO_DIVIDENDS), false);
});

test("an error shows the report sentence and not the loader message", () => {
  const rendered = html({ state: "error", message: "postgres password=secret" });
  assert.equal(text(rendered), DIVIDENDS_LOAD_ERROR);
  assert.equal(
    DIVIDENDS_LOAD_ERROR,
    "The stored dividends couldn't be loaded. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
  );
  assert.equal(rendered.includes("postgres password=secret"), false);
  assert.equal(rendered.includes(">—<"), false);
});

test("an empty list says no dividends are stored", () => {
  const rendered = html({ state: "ready", data: [] });
  assert.equal(rendered.includes(NO_DIVIDENDS), true);
  assert.equal(NO_DIVIDENDS, "No dividends stored.");
});

test("rows show symbol, ex-date, per-share amount and pay date in that order", () => {
  const rendered = html({ state: "ready", data: [ko] });
  assert.match(rendered, /<th[^>]*>Symbol<\/th>\s*<th[^>]*>Ex-date<\/th>\s*<th[^>]*>Amount<\/th>\s*<th[^>]*>Pay date<\/th>/);
  assert.match(rendered, /<td[^>]*>KO<\/td>\s*<td[^>]*>2024-03-14<\/td>\s*<td[^>]*>\$0\.485 per share<\/td>\s*<td[^>]*>2024-04-01<\/td>/);
  assert.equal(rendered.includes("·"), false);
});

test("a payload that is not a list shows the unreadable sentence", () => {
  const rendered = html({ state: "ready", data: { symbol: "KO" } as unknown as StoredDividend[] });
  assert.equal(text(rendered), DIVIDEND_LIST_UNREADABLE);
  assert.equal(
    DIVIDEND_LIST_UNREADABLE,
    "The dividend list couldn't be read. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
  );
  assert.equal(rendered.includes(NO_DIVIDENDS), false);
});
