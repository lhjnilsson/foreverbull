<template>
  <v-container v-if="strategy">
    <v-row>
      <v-col>
        <h1>Nothing to see here, yet :)</h1>
      </v-col>
      <v-col>
      </v-col>
    </v-row>
    <v-row>
      <v-col>
      </v-col>
    </v-row>
  </v-container>
</template>

<script>
import { getStrategy, updateStrategy } from "@/modules/strategies/backend";

export default {
  components: {
  },
  data() {
    return {
      strategy: null,
      services: [],
      backtest: null,
      worker: null,
    };
  },
  async created() {
    this.strategy = await getStrategy(this.$route.params.name);
  },
  methods: {
    async updateStrategy() {
      await updateStrategy(this.strategy.name)
    }
  },
  computed: {
  }
};
</script>

<style>
</style>
