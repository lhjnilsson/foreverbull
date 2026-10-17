import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import { Backtests } from "./Backtests";
import { Services } from "./Services";
import { Strategies } from "./Strategies";
import { useJson } from "./useJson";

function Wordmark() {
  return (
    <h1 className="wordmark">
      Foreverbull
      <svg
        viewBox="0 0 10 14"
        fill="none"
        aria-hidden="true"
        dangerouslySetInnerHTML={{
          __html: '<path d="M8 1H2v12h6" stroke="currentColor" stroke-width="1.25"/>',
        }}
      />
    </h1>
  );
}

function statusLine(ok: boolean | undefined, failed: boolean): string {
  if (failed) return "Start the broker.";
  if (ok === true) return "Broker answered.";
  return "";
}

export function App() {
  const health = useJson<{ ok?: boolean }>("/api/health");
  const failed = health.state === "error" || (health.state === "ready" && health.data.ok !== true);
  const ok = health.state === "ready" ? health.data.ok : undefined;

  return (
    <div className="shell">
      <div className="chrome">
        <header className="top">
          <Wordmark />
          <p className="status-line">{statusLine(ok, failed)}</p>
        </header>
        <nav className="areas" aria-label="Areas">
          <NavLink className="area" to="/services">
            Services
          </NavLink>
          <NavLink className="area" to="/backtests">
            Backtests
          </NavLink>
          <NavLink className="area" to="/strategies">
            Strategies
          </NavLink>
        </nav>
      </div>
      <main>
        <Routes>
          <Route path="/" element={<Navigate to="/services" replace />} />
          <Route path="/services" element={<Services />} />
          <Route path="/backtests/:name?" element={<Backtests />} />
          <Route path="/strategies/:name?" element={<Strategies />} />
          <Route path="*" element={<Navigate to="/services" replace />} />
        </Routes>
      </main>
    </div>
  );
}
