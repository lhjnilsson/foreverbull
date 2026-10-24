import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { clearUnauthorized, subscribeUnauthorized, unauthorizedSeen, type Me } from "./access";

export type AuthState =
  | { status: "loading" }
  | { status: "off" }
  | { status: "anonymous" }
  | { status: "in"; account: Me };

type AuthApi = {
  state: AuthState;
  signedIn: (account: Me) => void;
  signedOut: () => void;
};

const Ctx = createContext<AuthApi | null>(null);

function parseMe(value: unknown): Me | null {
  if (typeof value !== "object" || value === null) return null;
  const body = value as { id?: unknown; role?: unknown; login?: unknown; login_kind?: unknown };
  if (typeof body.id !== "string" || typeof body.role !== "string" || typeof body.login !== "string") return null;
  return {
    id: body.id,
    role: body.role,
    login: body.login,
    login_kind: typeof body.login_kind === "string" ? body.login_kind : "",
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: "loading" });
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const ctrl = new AbortController();
    fetch("/api/auth/me", { credentials: "same-origin", cache: "no-store", signal: ctrl.signal })
      .then(async (res) => {
        if (res.status === 401) {
          setState({ status: "anonymous" });
          return;
        }
        if (!res.ok) {
          setState({ status: "off" });
          return;
        }
        const account = parseMe(await res.json());
        if (!account) {
          setState({ status: "anonymous" });
          return;
        }
        clearUnauthorized();
        setState({ status: "in", account });
      })
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setState({ status: "off" });
      });
    return () => ctrl.abort();
  }, [revision]);

  useEffect(() => {
    return subscribeUnauthorized(() => {
      if (!unauthorizedSeen()) return;
      setState((current) => (current.status === "anonymous" ? current : { status: "anonymous" }));
    });
  }, []);

  function signedIn(account: Me) {
    clearUnauthorized();
    setState({ status: "in", account });
  }

  function signedOut() {
    setState({ status: "anonymous" });
    setRevision((current) => current + 1);
  }

  return <Ctx.Provider value={{ state, signedIn, signedOut }}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthApi {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("auth provider missing");
  return ctx;
}
