<template>
  <v-card>
    <v-card-title>
      Configuration
    </v-card-title>
    <v-select
      label="Backtest Service"
      :items="backtestServices"
      item-title="name"
      item-value="name"
      v-model="backtestService"
      return-object
      persistent-hint
    ></v-select>
    <v-select
      label="Worker Service"
      :items="workerServices"
      item-title="name"
      item-value="name"
      v-model="workerService"
      return-object
      persistent-hint
    ></v-select>
    <v-select
      v-model="calendar"
      :items="['XNYS', 'XNAS']"
      label="Calendar"
    ></v-select>
    <VueDatePicker
      range
      v-model="date"
      :enable-time-picker="false"
      format="yyyy/MM/dd"
      :auto-apply="true"
      :min-date="minDate"
    ></VueDatePicker>
    <br>
    <v-select
      v-model="symbols"
      :items="availableSymbols"
      label="Symbols"
      multiple
    ></v-select>
    <v-select
      v-model="benchmark"
      :items="availableSymbols"
      label="Benchmark"
    ></v-select>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-btn color="blue-darken-1" variant="text" @click="updateBacktest" :disabled="!configChanged">
        Update
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script>
import VueDatePicker from "@vuepic/vue-datepicker";
import "@vuepic/vue-datepicker/dist/main.css";
import { getAssets } from "@/modules/finance/backend";
import { getServices } from '@/modules/services/backend'
import { updateBacktest } from "@/modules/backtests/backend";

export default {
  components: {
    VueDatePicker,
  },
  props: {
    backtest: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      backtestServices: [],
      workerServices: [],
      backtestService: null,
      workerService: null,
      date: null,
      minDate: null,
      maxDate: null,
      availableSymbols: [],
      symbols: [],
      benchmark: null,
      calendar: null,
      storedSymbols: [],
    };
  },
  computed: {
    configChanged() {
      if (!this.backtest) {
        return false;
      }
      if (!this.backtestService || !this.workerService) {
        return false;
      }
      return (
        this.benchmark !== this.backtest.benchmark ||
        this.symbols !== this.backtest.symbols ||
        this.start !== this.backtest.start_time ||
        this.end !== this.backtest.end_time ||
        this.calendar !== this.backtest.calendar ||
        this.backtestService.name !== this.backtest.backtest_service ||
        this.workerService.name !== this.backtest.worker_service
      );
    },
  },
  methods: {
    async updateBacktest() {
      console.log(this.date)
      return
      await updateBacktest(this.backtest.name,
        this.backtestService.name,
        this.workerService.name,
        this.calendar,
        this.start,
        this.end,
        this.symbols,
        this.benchmark,
      );
      this.emit("updated")
    }
  },
  async mounted() {
    this.calendar = this.backtest.calendar;
    let start = this.backtest.start_time;
    let end = this.backtest.end_time;
    this.date = [new Date(start), new Date(end)];
    this.symbols = this.backtest.symbols;
    this.benchmark = this.backtest.benchmark;
    let assets = await getAssets();
    this.minDate = new Date(assets[0].start_ohlc);
    this.maxDate = new Date(assets[0].end_ohlc);
    this.availableSymbols = assets.map((asset) => asset.symbol);
    let services = await getServices();
    this.backtestServices = services.filter(
      (service) => service.type === "backtest"
    );
    this.workerServices = services.filter(
      (service) => service.type === "worker"
    );
    this.backtestService = this.backtestServices.find(
      (service) => service.name === this.backtest.backtest_service
    );
    this.workerService = this.workerServices.find(
      (service) => service.name === this.backtest.worker_service
    );
  },
};
</script>

<style>
</style>
