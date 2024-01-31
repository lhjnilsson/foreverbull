<template>
  <v-container>
    <v-card>
      <v-toolbar>
        <v-toolbar-title>Backtests</v-toolbar-title>
        <v-spacer></v-spacer>
        <add-backtest-dialog @added="backtestAdded" @error="errorAdded"></add-backtest-dialog>
      </v-toolbar>
      <v-alert v-if="addedStatusType.length > 0" closable :text="addedStatusText" :type="addedStatusType" variant="tonal"></v-alert>
      <v-table hover>
        <thead>
          <tr>
            <th class="text-left">Name</th>
            <th class="text-left">Backtest Service</th>
            <th class="text-left">Worker Service</th>
            <th class="text-left">Created</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in backtests" :key="item.name" @click="openBacktest(item)">
            <td>{{ item.name }}</td>
            <td>{{ item.backtest_service }}</td>
            <td>{{ item.worker_service }}</td>
            <td>{{ $filters.formatDate(item.created_at) }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
  </v-container>
</template>

<script>
import AddBacktestDialog from "@/modules/backtests/components/AddBacktestDialog.vue";
import { getBacktests } from '@/modules/backtests/backend';

export default {
  components: {
    AddBacktestDialog,
  },
  data() {
    return {
      backtests: [],
      addedStatusType: "",
      addedStatusText: null,
    }
  },
  methods: {
    async backtestAdded() {
      this.addedStatusType = "success";
      this.addedStatusText = "Backtest added successfully";
      this.backtests = await getBacktests();
    },
    errorAdded(error) {
      this.addedStatusType = "error";
      this.addedStatusText = `Error adding backtest: ${error.message}`;
    },
    openBacktest(backtest) {
      this.$router.push({ name: 'Backtest', params: { name: backtest.name } });
    }
  },
  async mounted() {
    this.backtests = await getBacktests();
    console.log(this.backtests)
  },
}
</script>

<style lang="scss">

</style>
