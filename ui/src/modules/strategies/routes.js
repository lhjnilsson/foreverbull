const routes = [
  {
    path: "",
    name: "Strategies",
    component: () => import("@/modules/strategies/views/ListStrategies.vue"),
  },
  {
    path: ":name",
    name: "Strategy",
    component: () => import("@/modules/strategies/views/Strategy.vue"),
  },
];

export { routes };
