<template>
  <v-container v-if="execution">
    <v-row>
      <v-col>
        <periods-chart :execution="execution"/>
      </v-col>
    </v-row>
    <v-row>
      <v-col>
        <order-table :orders=orders />
      </v-col>
    </v-row>
  </v-container>
</template>

<script>
import { getSession, getSessionExecutions, getExecution } from "@/modules/backtests/backend";
import OrderTable from "@/modules/finance/components/OrderTable.vue";
import PeriodsChart from "@/modules/strategies/components/PeriodsChart.vue";

export default {
  components: {
    OrderTable,
    PeriodsChart,
  },
  data() {
    return {
      session: null,
      executions: [],
      execution: null,
    }
  },
  async created() {
    this.session = await getSession(this.$route.params.sessionid);
    this.executions = await getSessionExecutions(this.$route.params.sessionid);
    if (this.executions.length > 0) {
      this.execution = await getExecution(this.executions[0].id);
    }
  }
}
</script>

<style>

</style>
