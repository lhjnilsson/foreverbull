const routes = [
  {
    path: "",
    name: "Services",
    component: () => import("@/modules/services/views/ListServices.vue"),
  },
  {
    path: ":name",
    name: "Service",
    component: () => import("@/modules/services/views/Service.vue"),
  },
];

export { routes };
