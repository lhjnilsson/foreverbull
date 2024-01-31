<template>
    <v-card>
      <v-toolbar>
      <v-toolbar-title>Period Results</v-toolbar-title>
      <v-spacer></v-spacer>
      <v-select :items="metrics" v-model="selectedMetrics" multiple></v-select>
    </v-toolbar>
    <apexchart type="line" :options="options" :series="series"></apexchart>
  </v-card>
</template>

<script>
import { getPeriods, getMetrics, getMetric } from "@/modules/backtests/backend";
export default {
  props: {
    execution: {
      type: Object,
      required: true,
    }
  },
  data() {
    return {
      periods: [],
      metrics: [],
      selectedMetrics: [],
      metric: [],
      options: {
        chart: {
          id: 'vuechart-example'
        },
        xaxis: {
          categories: this.periods,
        }
      },
      series: []
    }
  },
  methods: {
    async updateChartMetrics(n, old) {
      let added = n.filter(x => !old.includes(x));
      let removed = old.filter(x => !n.includes(x));
      if (removed.length > 0) {
        this.series = this.series.filter(x => !removed.includes(x.name));
      } else {
        let metric = await getMetric(this.execution.id, added[0]);
        metric = metric.reverse();
        this.series.push({
          name: added[0],
          data: metric,
        })
      }
    }
  },
  watch: {
    async selectedMetrics (n, old) {
      await this.updateChartMetrics(n, old);
    }
  },
  async created() {
    this.periods = await getPeriods(this.execution.id);
    this.metrics = await getMetrics(this.execution.id);
    this.metrics.sort();
    if (this.metrics.includes('portfolio_value')) {
      this.selectedMetrics.push('portfolio_value');
      await this.updateChartMetrics(this.selectedMetrics, []);
    }
  }
}
</script>

<style>

</style>
