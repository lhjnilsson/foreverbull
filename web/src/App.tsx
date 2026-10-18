import { Navigate, Route, Routes } from "react-router-dom";
import { Sidebar } from "./components/fb";
import { BacktestDetail, BacktestIndex, BacktestsLayout } from "./routes/backtests";
import { ServicesPage } from "./routes/services";
import { StrategiesLayout, StrategyDetail, StrategyIndex } from "./routes/strategies";

export function App() {
  return (
    <div className="flex h-screen w-full overflow-hidden">
      <Sidebar />
      <main className="flex h-full min-w-0 flex-1">
        <Routes>
          <Route path="/" element={<Navigate to="/services" replace />} />
          <Route path="/services" element={<ServicesPage />} />
          <Route path="/backtests" element={<BacktestsLayout />}>
            <Route index element={<BacktestIndex />} />
            <Route path=":name" element={<BacktestDetail />} />
          </Route>
          <Route path="/strategies" element={<StrategiesLayout />}>
            <Route index element={<StrategyIndex />} />
            <Route path=":name" element={<StrategyDetail />} />
          </Route>
          <Route path="*" element={<Navigate to="/services" replace />} />
        </Routes>
      </main>
    </div>
  );
}
