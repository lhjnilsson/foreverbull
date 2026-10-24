import { NavLink } from "react-router-dom";
import { useEffect, useId, useState, type ButtonHTMLAttributes, type ReactNode } from "react";
import { Boxes, FlaskConical, LogOut, Plus, Target, X } from "lucide-react";
import { useAuth } from "../auth-context";
import type { DateParts } from "../types";
import { postJson, useJson } from "../useJson";

const nav = [
  { to: "/services", label: "Services", icon: Boxes },
  { to: "/backtests", label: "Backtests", icon: FlaskConical },
  { to: "/strategies", label: "Strategies", icon: Target },
] as const;

export function Sidebar() {
  const health = useJson<{ ok?: boolean }>("/api/health");
  const { state, signedOut } = useAuth();
  const [leaving, setLeaving] = useState(false);
  const answered = health.state === "ready" && health.data.ok === true;

  async function logOut() {
    if (leaving) return;
    setLeaving(true);
    await postJson("/api/auth/logout", {});
    signedOut();
  }

  return (
    <aside className="flex h-full w-60 shrink-0 flex-col border-r bg-card">
      <div className="px-5 py-6 text-lg font-semibold text-foreground">Foreverbull</div>
      <nav className="flex flex-col px-2">
        {nav.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-sm px-3 py-2.5 transition-colors duration-150 hover:text-foreground ${
                isActive ? "bg-primary-soft font-medium !text-primary" : "text-muted-foreground"
              }`
            }
          >
            <item.icon size={16} aria-hidden />
            {item.label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto flex flex-col items-start gap-2 px-5 py-4 text-xs text-muted-foreground">
        {state.status === "in" ? (
          <div className="flex w-full items-center justify-between gap-2">
            <NavLink to="/account" className="min-w-0 truncate font-mono text-[13px] hover:text-foreground">
              {state.account.login}
            </NavLink>
            <IconButton label="Log out" disabled={leaving} onClick={() => void logOut()}>
              <LogOut size={16} aria-hidden />
            </IconButton>
          </div>
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
  count,
  newLabel,
  newTitle,
  newForm,
  allowCreate = true,
  children,
}: {
  title: string;
  count?: number;
  newLabel: string;
  newTitle: string;
  newForm: (close: () => void) => ReactNode;
  allowCreate?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <section className="flex h-full min-h-0 w-80 shrink-0 flex-col overflow-hidden border-r bg-background">
      <header className="flex h-16 shrink-0 items-center justify-between border-b px-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-base font-semibold">{title}</h1>
          {count !== undefined ? <span className="font-mono text-xs text-muted-foreground">{count}</span> : null}
        </div>
        {allowCreate ? (
          <IconButton label={newLabel} onClick={() => setOpen(true)}>
            <Plus size={16} aria-hidden />
          </IconButton>
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
  const titleId = useId();
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-overlay backdrop-blur-sm" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={`animate-enter absolute left-1/2 top-1/2 max-h-[90vh] w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 overflow-auto rounded-md border bg-card shadow-report ${
          wide ? "max-w-6xl" : "max-w-md"
        }`}
      >
        <header className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b bg-card px-6 py-4">
          <h2 id={titleId} className="min-w-0 break-all text-base font-semibold">
            {title}
          </h2>
          <IconButton label="Close" onClick={onClose}>
            <X size={16} aria-hidden />
          </IconButton>
        </header>
        <div className="p-6">{children}</div>
      </div>
    </div>
  );
}

export function IconButton({
  label,
  children,
  className,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return (
    <button
      {...props}
      type="button"
      aria-label={label}
      title={label}
      className={`shrink-0 rounded-sm p-1 text-muted-foreground transition-colors duration-150 hover:text-primary ${className ?? ""}`}
    >
      {children}
    </button>
  );
}

export const rowClass =
  "block border-b border-l-2 border-l-transparent px-5 py-4 transition-colors duration-150 hover:bg-card";
export const rowActive = "!border-l-primary bg-primary-soft";

export function Empty({ text }: { text: string }) {
  return <li className="px-5 py-6 text-muted-foreground">{text}</li>;
}

export function Detail({ k, children }: { k: string; children?: ReactNode }) {
  return (
    <section key={k} className="animate-enter h-full min-h-0 flex-1 overflow-auto bg-card px-8 py-8 lg:px-10">
      {children}
    </section>
  );
}

export function DetailHeading({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <header className="mb-8 flex flex-wrap items-start justify-between gap-4 border-b pb-6">
      <div className="min-w-0">
        <h1 className="break-all font-mono text-2xl font-medium leading-snug">{title}</h1>
        {subtitle ? <div className="mt-2 text-sm text-muted-foreground">{subtitle}</div> : null}
      </div>
      {children}
    </header>
  );
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[7rem_minmax(0,1fr)] gap-4 py-2.5 sm:grid-cols-[9rem_minmax(0,1fr)]">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="break-words font-mono text-[13px]">{children}</dd>
    </div>
  );
}

export function H2({ children }: { children: ReactNode }) {
  return <h2 className="mb-3 mt-9 text-xs font-semibold uppercase text-muted-foreground">{children}</h2>;
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
  return (
    <span className={`inline-flex items-center gap-2 font-mono text-xs ${statusColor[s] ?? ""}`}>
      <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-current" />
      {s}
    </span>
  );
}

export function FormError({ message }: { message: string }) {
  if (!message) return null;
  return <p className="mt-2 text-status-error">{message}</p>;
}
