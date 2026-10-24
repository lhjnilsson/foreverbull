export type Me = {
  id: string;
  role: string;
  login: string;
  login_kind: string;
};

export function showTokenMint(me: Me | null): boolean {
  return me?.role === "editor" || me?.role === "admin";
}

export function canCreate(status: string, role: string | undefined): boolean {
  if (status === "off") return true;
  return role === "editor" || role === "admin";
}

export function canGrant(me: Me | null, ownerLogin: string | undefined): boolean {
  if (!me) return false;
  if (me.role === "admin") return true;
  if (me.role !== "editor") return false;
  const owner = ownerLogin?.trim() ?? "";
  const login = me.login.trim();
  return owner !== "" && owner === login;
}

let unauthorized = false;
const listeners = new Set<() => void>();

export function noteUnauthorized(): void {
  unauthorized = true;
  for (const listener of listeners) listener();
}

export function clearUnauthorized(): void {
  unauthorized = false;
  for (const listener of listeners) listener();
}

export function unauthorizedSeen(): boolean {
  return unauthorized;
}

export function subscribeUnauthorized(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
