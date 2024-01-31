<template>
  <v-row justify="center">
    <v-dialog
      v-model="dialog"
      persistent
      width="480"
    >
      <template v-slot:activator="{ props }">
        <v-btn
          color="primary"
          v-bind="props"
        >
          Add Backtest
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="text-h5">Add new Backtest</span>
        </v-card-title>
        <v-card-text>
          <v-container>
            <v-form>
              <v-row>
                <v-col cols="12" md="12">
                  <v-text-field
                    label="Backtest Name"
                    v-model="name"
                  ></v-text-field>
                  <v-card-subtitle>Configuration</v-card-subtitle>
                  <v-select
                    label="Backtest Service"
                    :items="backtestServices"
                    item-title="name"
                    item-value="name"
                    :hint="`${backtestService.type}, ${backtestService.image}`"
                    v-model="backtestService"
                    return-object
                    persistent-hint
                  ></v-select>
                  <v-select
                    v-if="workerService"
                    label="Worker Service"
                    :items="workerServices"
                    item-title="name"
                    item-value="name"
                    :hint="`${workerService.type}, ${workerService.image}`"
                    v-model="workerService"
                    return-object
                    persistent-hint
                  ></v-select>
                  <v-select
                    v-else
                    disabled
                    label="Worker Service"
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
                    :items="storedSymbols"
                    label="Symbols"
                    multiple
                  ></v-select>
                  <v-select
                    v-model="benchmark"
                    :items="storedSymbols"
                    label="Benchmark"
                  ></v-select>

                </v-col>
              </v-row>
            </v-form>
          </v-container>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn
            color="blue-darken-1"
            variant="text"
            @click="dialog = false"
          >
            Abort
          </v-btn>
          <v-btn
            color="blue-darken-1"
            variant="text"
            @click="createBacktest"
          >
            Create
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-row>
</template>

<script>
import { createBacktest } from '@/modules/backtests/backend'
import { getServices } from '@/modules/services/backend'
import { getAssets } from "@/modules/finance/backend";
import VueDatePicker from "@vuepic/vue-datepicker";
import "@vuepic/vue-datepicker/dist/main.css";

export default {
  components: {
    VueDatePicker,
  },
  data: () => ({
    dialog: false,
    services: [],
    name: null,
    backtestServices: [],
    workerServices: [],
    backtestService: null,
    workerService: null,

    assets: [],
    storedSymbols: [],
    symbols: [],
    benchmark: null,
    date: null,
    minDate: null,
    maxDate: null,
    calendar: "XNYS",
  }),
  methods: {
    async createBacktest() {
      console.log("Symbols: ", this.symbols)
      try {
        let backtest = await createBacktest(
          this.name,
          this.backtestService.name,
          this.workerService ? this.workerService.name : null,
          this.calendar,
          this.date[0],
          this.date[1],
          this.benchmark,
          this.symbols,
        )
        this.$emit('added', backtest)
      } catch (error) {
        console.log(error)
        this.$emit('error', error)
      }
      this.dialog = false
    }
  },
  async mounted() {

    let assets = await getAssets();
    this.minDate = new Date(assets[0].start_ohlc);
    this.maxDate = new Date(assets[0].end_ohlc);
    this.date = [this.minDate, this.maxDate];
    this.symbols = assets.map((asset) => asset.symbol);
    let services = await getServices();
    this.backtestServices = services.filter(
      (service) => service.type === "backtest"
    );
    this.workerServices = services.filter(
      (service) => service.type === "worker"
    );

    this.backtestService = this.backtestServices[0];
    this.workerService = this.workerServices[0];
  }
}
</script>

<style>

</style>
