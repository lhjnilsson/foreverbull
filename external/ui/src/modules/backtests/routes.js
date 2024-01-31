const routes = [
  {
    path: "",
    name: "Backtests",
    component: () => import("@/modules/backtests/views/ListBacktests.vue"),
  },
  {
    path: ":name",
    name: "Backtest",
    component: () => import("@/modules/backtests/views/Backtest.vue"),
  },
  {
    path: ":name/sessions/:sessionid",
    name: "Session",
    component: () => import("@/modules/backtests/views/Session.vue"),
  },
  {
    path: ":id",
    name: "Execution",
    component: () => import("@/modules/backtests/views/Execution.vue"),
  },
];

export { routes };
