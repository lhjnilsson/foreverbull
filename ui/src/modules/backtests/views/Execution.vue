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
import OrderTable from "@/modules/finance/components/OrderTable.vue";
import PeriodsChart from "@/modules/strategies/components/PeriodsChart.vue";

import { getExecution, getOrders } from "@/modules/backtests/backend";

export default {
  components: {
    OrderTable,
    PeriodsChart,
  },
  data() {
    return {
      execution: null,
      orders: []
    }
  },
  async created() {
    this.execution = await getExecution(this.$route.params.id);
    this.orders = await getOrders(this.$route.params.id);
  }
}
</script>

<style>

</style>
