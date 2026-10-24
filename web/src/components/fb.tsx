import { NavLink } from "react-router-dom";
import { useEffect, useState, type ReactNode } from "react";
import { Boxes, FlaskConical, Plus, Target, X } from "lucide-react";
import { useAuth } from "../auth-context";
import type { DateParts } from "../types";
import { useJson } from "../useJson";

const nav = [
  { to: "/services", label: "Services", icon: Boxes },
  { to: "/backtests", label: "Backtests", icon: FlaskConical },
  { to: "/strategies", label: "Strategies", icon: Target },
] as const;

export function Sidebar() {
  const health = useJson<{ ok?: boolean }>("/api/health");
  const { state } = useAuth();
  const answered = health.state === "ready" && health.data.ok === true;
  return (
    <aside className="flex h-full w-60 shrink-0 flex-col border-r bg-card">
      <div className="px-5 py-5 text-[15px] font-semibold tracking-tight text-foreground">Foreverbull</div>
      <nav className="flex flex-col px-2">
        {nav.map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            className={({ isActive }) =>
              `flex items-center gap-2.5 rounded-sm px-3 py-1.5 transition-colors duration-150 hover:text-foreground ${
                isActive ? "bg-primary-soft font-medium !text-primary" : "text-muted-foreground"
              }`
            }
          >
            <n.icon size={14} aria-hidden />
            {n.label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto flex flex-col items-start gap-2 px-5 py-4 text-xs text-muted-foreground">
        {state.status === "in" ? (
          <NavLink to="/account" className="truncate font-mono text-[13px] hover:text-foreground">
            {state.account.login}
          </NavLink>
        ) : null}
        {answered ? (
          <span
            role="img"
            aria-label="Broker answered"
            title="Broker answered"
            className="h-1.5 w-1.5 rounded-full bg-status-ready"
          />
        ) : null}
      </div>
    </aside>
  );
}

export function ListPane({
  title,
  newLabel,
  newTitle,
  newForm,
  allowCreate = true,
  children,
}: {
  title: string;
  newLabel: string;
  newTitle: string;
  newForm: (close: () => void) => ReactNode;
  allowCreate?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <section className="flex h-full min-h-0 w-80 shrink-0 flex-col overflow-hidden border-r">
      <header className="flex h-12 items-center justify-between border-b px-4">
        <h1 className="font-medium">{title}</h1>
        {allowCreate ? (
          <button
            type="button"
            aria-label={newLabel}
            title={newLabel}
            onClick={() => setOpen(true)}
            className="rounded-sm p-1 text-muted-foreground transition-colors duration-150 hover:text-primary"
          >
            <Plus size={16} aria-hidden />
          </button>
        ) : null}
      </header>
      <ul className="min-h-0 flex-1 overflow-auto">{children}</ul>
      {allowCreate ? (
        <Modal open={open} onClose={() => setOpen(false)} title={newTitle}>
          {newForm(() => setOpen(false))}
        </Modal>
      ) : null}
    </section>
  );
}

export function Modal({
  open,
  onClose,
  title,
  wide,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  wide?: boolean;
  children: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div role="dialog" aria-modal="true" aria-label={title} className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />
      <div
        className={`animate-enter relative max-h-[85vh] w-full ${wide ? "max-w-4xl" : "max-w-md"} overflow-auto rounded-sm border bg-card p-6 shadow-lg`}
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="font-medium">{title}</h2>
          <button
            type="button"
            aria-label="Close"
            title="Close"
            onClick={onClose}
            className="rounded-sm p-1 text-muted-foreground transition-colors duration-150 hover:text-foreground"
          >
            <X size={16} aria-hidden />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export const rowClass =
  "block border-b border-l-2 border-l-transparent px-4 py-2.5 transition-colors duration-150 hover:bg-card";
export const rowActive = "!border-l-primary bg-card";

export function Empty({ text }: { text: string }) {
  return <li className="px-4 py-6 text-muted-foreground">{text}</li>;
}

export function Detail({ k, children }: { k: string; children?: ReactNode }) {
  return (
    <section key={k} className="animate-enter h-full min-h-0 flex-1 overflow-auto bg-card px-8 py-6">
      {children}
    </section>
  );
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[9rem_1fr] gap-4 py-1.5">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-mono text-[13px]">{children}</dd>
    </div>
  );
}

export function H2({ children }: { children: ReactNode }) {
  return (
    <h2 className="mb-2 mt-8 text-xs font-medium uppercase tracking-wide text-muted-foreground">{children}</h2>
  );
}

export const inputClass =
  "w-full rounded-sm border bg-card px-2.5 py-1.5 font-mono text-[13px] outline-none focus:border-primary";
export const buttonClass =
  "rounded-sm bg-primary px-3 py-1.5 text-primary-foreground transition-opacity duration-150 hover:opacity-90 disabled:opacity-50";

export function fmtDate(d?: DateParts) {
  if (!d || d.year == null || d.month == null || d.day == null) return "—";
  return `${d.year}-${String(d.month).padStart(2, "0")}-${String(d.day).padStart(2, "0")}`;
}

export function fmtTime(s?: string) {
  if (!s) return "—";
  return s.replace("T", " ").replace(/\.\d+Z$|Z$/, "Z");
}

const statusColor: Record<string, string> = {
  READY: "text-status-ready",
  CREATED: "text-status-pending",
  INTERVIEW: "text-status-pending",
  ERROR: "text-status-error",
};

export function StatusText({ s }: { s?: string }) {
  if (!s) return null;
  return <span className={`font-mono text-xs ${statusColor[s] ?? ""}`}>{s}</span>;
}

export function FormError({ message }: { message: string }) {
  if (!message) return null;
  return <p className="mt-2 text-status-error">{message}</p>;
}
