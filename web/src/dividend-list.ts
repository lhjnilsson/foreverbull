import { createElement, type ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import {
  DIVIDEND_LIST_UNREADABLE,
  DIVIDENDS_LOAD_ERROR,
  NO_DIVIDENDS,
  dividendLines,
  type StoredDividend,
} from "./dividends.ts";
import type { Load } from "./useJson.ts";

const cell = "py-2 pr-6";

function textRow(children: ReactNode) {
  return createElement("p", { className: "text-muted-foreground" }, children);
}

export function DividendList({ load }: { load: Load<StoredDividend[]> }) {
  if (load.state === "idle" || load.state === "loading") {
    return createElement(
      "p",
      { className: "flex items-center gap-2 text-muted-foreground" },
      createElement(LoaderCircle, { className: "animate-spin motion-reduce:animate-none", size: 16, "aria-hidden": true }),
      "Loading the stored dividends.",
    );
  }
  if (load.state === "error") return textRow(DIVIDENDS_LOAD_ERROR);
  if (!Array.isArray(load.data)) return textRow(DIVIDEND_LIST_UNREADABLE);
  const lines = dividendLines(load.data);
  if (lines.length === 0) return textRow(NO_DIVIDENDS);
  return createElement(
    "table",
    { className: "w-full text-left font-mono text-[13px]" },
    createElement(
      "thead",
      null,
      createElement(
        "tr",
        { className: "text-xs text-muted-foreground" },
        createElement("th", { className: `${cell} font-normal` }, "Symbol"),
        createElement("th", { className: `${cell} font-normal` }, "Ex-date"),
        createElement("th", { className: `${cell} font-normal` }, "Amount"),
        createElement("th", { className: "py-2 font-normal" }, "Pay date"),
      ),
    ),
    createElement(
      "tbody",
      null,
      lines.map((line) =>
        createElement(
          "tr",
          { key: `${line.symbol}\u0000${line.exDate}`, className: "border-t" },
          createElement("td", { className: cell }, line.symbol),
          createElement("td", { className: cell }, line.exDate),
          createElement("td", { className: cell }, line.amount),
          createElement("td", { className: "py-2" }, line.payDate),
        ),
      ),
    ),
  );
}
