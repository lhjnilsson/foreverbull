import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import { Backtests } from "./Backtests";
import { Services } from "./Services";
import { Strategies } from "./Strategies";
import { useJson } from "./useJson";

function Wordmark() {
  return (
    <h1 className="wordmark">
      Foreverbull
      <svg viewBox="0 0 14 14" fill="none" aria-hidden="true">
        <path
          d="M1 4.5V1h3.5M9.5 1H13v3.5M13 9.5V13H9.5M4.5 13H1V9.5"
          stroke="currentColor"
          strokeWidth="1.25"
        />
      </svg>
    </h1>
  );
}

function statusLine(ok: boolean | undefined): string {
  if (ok === true) return "Broker answered";
  return "";
}

export function App() {
  const health = useJson<{ ok?: boolean }>("/api/health");
  const line = statusLine(health.state === "ready" ? health.data.ok : undefined);

  return (
    <div className="shell">
      <div className="chrome">
        <header className="top">
          <Wordmark />
          {line ? <p className="status-line">{line}</p> : null}
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
