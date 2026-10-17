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

export function useJson<T>(url: string | null): Load<T> {
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
  }, [url]);

  return load;
}
