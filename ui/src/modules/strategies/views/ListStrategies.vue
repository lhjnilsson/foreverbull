<template>
  <v-container>
    <v-card>
      <v-toolbar>
      <v-toolbar-title>Strategies</v-toolbar-title>
      <v-spacer></v-spacer>
      <add-strategy-dialog @added="strategyAdded" @error="errorAdded"></add-strategy-dialog>
    </v-toolbar>
    <v-alert v-if="addedStatusType.length > 0" closable :text="addedStatusText" :type="addedStatusType" variant="tonal"></v-alert>
    <v-table hover>
      <thead>
        <tr>
          <th class="text-left">Name</th>
          <th class="text-left">Backtest</th>
          <th class="text-left">Created</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in strategies" :key="item.name" @click="openStrategy(item)">
          <td>{{ item.name }}</td>
          <td>{{ item.backtest }}</td>
          <td>{{ item.created_at }}</td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
  </v-container>
</template>

<script>
import AddStrategyDialog from "@/modules/strategies/components/AddStrategyDialog.vue";
import { getStrategies } from '@/modules/strategies/backend'

export default {
  components: {
    AddStrategyDialog,
  },
  data() {
    return {
      strategies: [],
      addedStatusType: "",
      addedStatusText: null,
    }
  },
  methods: {
    async strategyAdded() {
      this.addedStatusType = "success";
      this.addedStatusText = "Strategy added successfully";
      this.strategies = await getStrategies();
    },
    errorAdded(error) {
      this.addedStatusType = "error";
      this.addedStatusText = `Error adding strategy: ${error.message}`;
    },
    openStrategy(strategy) {
      this.$router.push({ name: 'Strategy', params: { name: strategy.name } });
    }
  },
  async mounted() {
    this.strategies = await getStrategies();
  }
};
</script>

<style>
</style>
