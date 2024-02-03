import { defineStore } from "pinia";
import { getStrategies } from "./backend";

export const useStrategiesStore = defineStore("strategies", {
  state: () => ({
    strategies: [],
  }),
  actions: {
    async getStrategies() {
      this.strategies = await getStrategies();
    },
  },
});
