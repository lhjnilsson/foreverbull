import assert from "node:assert/strict";
import test from "node:test";

import { canGrant, showTokenMint, type Me } from "./src/access.ts";

function me(role: string, login = "ada"): Me {
  return { id: "1", role, login, login_kind: "password" };
}

test("a viewer response does not show a token mint control", () => {
  const response = { id: "1", role: "viewer", login: "ada", login_kind: "password" };
  assert.equal(showTokenMint(response), false);
  assert.equal(showTokenMint(me("pending")), false);
  assert.equal(showTokenMint(me("none")), false);
  assert.equal(showTokenMint(me("editor")), true);
  assert.equal(showTokenMint(me("admin")), true);
  assert.equal(showTokenMint(null), false);
});

test("grant controls are the owning editor or an admin on every row", () => {
  assert.equal(canGrant(me("viewer"), "ada"), false);
  assert.equal(canGrant(me("editor", "ada"), "ada"), true);
  assert.equal(canGrant(me("editor", "ada"), "other"), false);
  assert.equal(canGrant(me("editor", "ada"), ""), false);
  assert.equal(canGrant(me("admin", "root"), "other"), true);
  assert.equal(canGrant(null, "ada"), false);
});
