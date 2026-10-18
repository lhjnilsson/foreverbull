import { useEffect, useState } from "react";

export type Load<T> =
  | { state: "idle" }
  | { state: "loading" }
  | { state: "ready"; data: T }
  | { state: "error"; message: string };

class HttpError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "HttpError";
  }
}

function isAbort(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

async function readError(res: Response): Promise<string> {
  try {
    const body: unknown = await res.json();
    if (typeof body === "object" && body !== null && "error" in body) {
      const message = (body as { error: unknown }).error;
      if (typeof message === "string") return message;
    }
  } catch {
    return "";
  }
  return "";
}

export function useJson<T>(url: string | null, revision = 0): Load<T> {
  const [load, setLoad] = useState<Load<T>>({ state: "loading" });

  useEffect(() => {
    if (url === null) {
      setLoad({ state: "idle" });
      return;
    }
    const ctrl = new AbortController();
    setLoad({ state: "loading" });
    fetch(url, { signal: ctrl.signal, cache: "no-store" })
      .then(async (res) => {
        if (!res.ok) {
          throw new HttpError(await readError(res));
        }
        return (await res.json()) as T;
      })
      .then((data) => {
        if (!ctrl.signal.aborted) setLoad({ state: "ready", data });
      })
      .catch((err: unknown) => {
        if (ctrl.signal.aborted || isAbort(err)) return;
        const message = err instanceof HttpError ? err.message : "";
        setLoad({ state: "error", message });
      });
    return () => ctrl.abort();
  }, [url, revision]);

  return load;
}

export type PostResult = { ok: true; data: unknown } | { ok: false; message: string };

export async function postJson(url: string, body: unknown): Promise<PostResult> {
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      cache: "no-store",
    });
  } catch {
    return { ok: false, message: "" };
  }
  if (!res.ok) {
    return { ok: false, message: await readError(res) };
  }
  try {
    return { ok: true, data: (await res.json()) as unknown };
  } catch {
    return { ok: false, message: "" };
  }
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== "object" || value === null) return null;
  return value as Record<string, unknown>;
}

export function readString(value: unknown, key: string): string | null {
  const item = asRecord(value);
  if (!item) return null;
  const field = item[key];
  if (typeof field !== "string" || field.trim() === "") return null;
  return field;
}
