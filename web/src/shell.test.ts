import assert from "node:assert/strict";
import { test } from "node:test";
import { docsPath, homePath, overviewPath, shellNav } from "./shell.ts";

test("the signed-in home is overview", () => {
  assert.equal(overviewPath, "/overview");
  assert.equal(homePath, overviewPath);
  assert.equal(shellNav[0]?.to, homePath);
  assert.equal(shellNav[0]?.label, "Overview");
});

test("docs sits in the shell with the existing sections", () => {
  assert.deepEqual(
    shellNav.map((item) => ({ to: item.to, label: item.label })),
    [
      { to: "/overview", label: "Overview" },
      { to: docsPath, label: "Docs" },
      { to: "/services", label: "Services" },
      { to: "/backtests", label: "Backtests" },
      { to: "/strategies", label: "Strategies" },
    ],
  );
});
