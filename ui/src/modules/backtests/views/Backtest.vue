<template>
  <v-container>
    <v-row v-if="backtest">
    <v-col>
      <configuration :backtest="backtest" @updated="getBacktest"></configuration>
    </v-col>
    <v-col>
      <create-execution :backtest="backtest" @sessionCreated="sessionCreated"></create-execution>
    </v-col>
    </v-row>
    <v-row v-if="backtest">
      <v-col>
        <sessions-table :sessions="sessions" @session="(session) => gotoSession(session)"></sessions-table>
      </v-col>
    </v-row>
  </v-container>
</template>

<script>
import SessionsTable from "@/modules/backtests/components/SessionsTable.vue";
import Configuration from "@/modules/backtests/components/Configuration.vue";
import CreateExecution from "@/modules/backtests/components/CreateExecution.vue";
import { getBacktest, getSessions } from '@/modules/backtests/backend';

export default {
  components: {
    SessionsTable,
    Configuration,
    CreateExecution
  },
  data() {
    return {
      backtest: null,
      sessions: [],
    }
  },
  methods: {
    async getBacktest() {
      this.backtest = await getBacktest(this.$route.params.name);
    },
    sessionCreated() {
      console.log("session created")
    },
    gotoSession(session) {
      console.log(session)

      this.$router.push({ name: "Session", params: { name: session.backtest, sessionid: session.id } });
    },
  },
  async mounted() {
    await this.getBacktest();
    this.sessions = await getSessions(this.backtest.name);
  },
}
</script>

<style>

</style>
