import { Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AccountPage, Locked, LoginPage } from "./auth";
import { AuthProvider, useAuth } from "./auth-context";
import { Sidebar } from "./components/fb";
import { BacktestDetail, BacktestIndex, BacktestsLayout } from "./routes/backtests";
import { DocsPage } from "./routes/docs";
import { OverviewPage } from "./routes/overview";
import { ServicesPage } from "./routes/services";
import { StrategiesLayout, StrategyDetail, StrategyIndex } from "./routes/strategies";
import { backtestsPath, docsPath, homePath, overviewPath, servicesPath, strategiesPath } from "./shell";

function Shell() {
  const { state } = useAuth();
  if (state.status === "loading") return <div className="h-screen" />;
  if (state.status === "anonymous") return <Navigate to="/login" replace />;
  if (state.status === "in" && (state.account.role === "pending" || state.account.role === "none")) {
    return <Locked account={state.account} />;
  }
  return (
    <div className="flex h-screen w-full overflow-hidden">
      <Sidebar />
      <main className="flex h-full min-w-0 flex-1">
        <Outlet />
      </main>
    </div>
  );
}

export function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<Shell />}>
          <Route path="/" element={<Navigate to={homePath} replace />} />
          <Route path={overviewPath} element={<OverviewPage />} />
          <Route path={docsPath} element={<DocsPage />} />
          <Route path={servicesPath} element={<ServicesPage />} />
          <Route path={backtestsPath} element={<BacktestsLayout />}>
            <Route index element={<BacktestIndex />} />
            <Route path=":name" element={<BacktestDetail />} />
          </Route>
          <Route path={strategiesPath} element={<StrategiesLayout />}>
            <Route index element={<StrategyIndex />} />
            <Route path=":name" element={<StrategyDetail />} />
          </Route>
          <Route path="/account" element={<AccountPage />} />
          <Route path="*" element={<Navigate to={homePath} replace />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
