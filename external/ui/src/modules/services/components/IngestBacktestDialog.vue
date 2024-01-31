<template>
  <v-row justify="center">
    <v-dialog v-model="dialog" persistent width="480">
      <template v-slot:activator="{ props }">
        <v-btn color="primary" v-bind="props"> Ingest Backtest </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="text-h5">Ingest backtest data</span>
        </v-card-title>
        <v-card-text>
          <v-container>
            <v-form>
              <v-row>
                <v-col cols="12" md="12">
                  <VueDatePicker
                    v-model="start"
                    :enable-time-picker="false"
                    format="yyyy/MM/dd"
                    :auto-apply="true"
                  ></VueDatePicker>
                </v-col>
                <v-col cols="12" md="12">
                  <VueDatePicker
                    v-model="end"
                    :enable-time-picker="false"
                    format="yyyy/MM/dd"
                    :auto-apply="true"
                  ></VueDatePicker>
                </v-col>
                <v-col cols="12" md="12">
                  <v-select
                    v-model="symbols"
                    :items="storedSymbols"
                    label="Symbols"
                    multiple
                  ></v-select>
                </v-col>
                <v-col cols="12" md="12"
                  ><v-select
                    v-model="benchmark"
                    :items="storedSymbols"
                    label="Benchmark"
                  ></v-select
                ></v-col>
                <v-col cols="12" md="12">
                  <v-select
                    v-model="calendar"
                    :items="['XNYS', 'XNAS']"
                    label="Calendar"
                  ></v-select>
                </v-col>
              </v-row>
            </v-form>
          </v-container>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="blue-darken-1" variant="text" @click="dialog = false">
            Abort
          </v-btn>
          <v-btn color="blue-darken-1" variant="text" @click="ingest">
            Ingest
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-row>
</template>

<script>
import VueDatePicker from "@vuepic/vue-datepicker";
import "@vuepic/vue-datepicker/dist/main.css";
import { getAssets } from "@/modules/finance/backend";
import { ingestBacktest } from "@/modules/services/backend";

export default {
  props: {
    service: {
      type: Object,
      required: true,
    },
  },
  components: {
    VueDatePicker,
  },
  data: () => ({
    dialog: false,
    assets: [],
    storedSymbols: [],
    symbols: [],
    benchmark: null,
    start: null,
    end: null,
    calendar: "XNYS",
  }),
  methods: {
    async ingest() {
      console.log("Service: ", this.service);
      try {
        await ingestBacktest(
          this.service.name,
          this.start,
          this.end,
          this.calendar,
          this.symbols,
          this.benchmark
        );
        this.dialog = false;
      } catch (error) {
        console.log("ERR: ", error.message);
      }
      this.dialog = false;
    },
  },
  async created() {
    this.assets = await getAssets();
    this.storedSymbols = this.assets.map((asset) => asset.symbol);
    this.symbols = this.storedSymbols;
    this.start = this.assets[0].start_ohlc;
    this.end = this.assets[0].end_ohlc;
  },
};
</script>

<style>
</style>
