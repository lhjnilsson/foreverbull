import { defineStore } from "pinia";
import { getServices } from "./backend";

export const useServicesStore = defineStore("services", {
  state: () => ({
    services: [],
  }),
  actions: {
    async getServices() {
      this.services = await getServices();
    },
  },
});
