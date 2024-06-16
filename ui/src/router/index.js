// Composables
import { createRouter, createWebHistory } from "vue-router";
import { routes as backtestRoutes } from "@/modules/backtests/routes.js";
import { routes as servicesRoutes } from "@/modules/services/routes.js";
import { routes as strategiesRoutes } from "@/modules/strategies/routes.js";
import { routes as financeRoutes } from "@/modules/finance/routes.js";

const routes = [
  {
    path: "/",
    component: () => import("@/layouts/Layout.vue"),
    children: [
      {
        path: "",
        name: "Dashboard",
        component: () => import("@/views/Home.vue"),
      },
    ],
  },
  {
    path: "/backtests",
    component: () => import("@/layouts/Layout.vue"),
    children: backtestRoutes,
  },
  {
    path: "/services",
    component: () => import("@/layouts/Layout.vue"),
    children: servicesRoutes,
  },
  {
    path: "/strategies",
    component: () => import("@/layouts/Layout.vue"),
    children: strategiesRoutes,
  },
  {
    path: "/finance",
    component: () => import("@/layouts/Layout.vue"),
    children: financeRoutes,
  },
];

const router = createRouter({
  history: createWebHistory(process.env.BASE_URL),
  routes,
});

export default router;
