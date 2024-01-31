<template>
  <v-card>
    <template v-slot:title>Backtest Configuration</template>
    <template v-slot:text>
      <v-list>
        <VueDatePicker
          v-model="updatedStrategy.backtest_config.start"
          :enable-time-picker="false"
          format="yyyy/MM/dd"
          :auto-apply="true"
        ></VueDatePicker>
        <v-list-item-subtitle> Start Time </v-list-item-subtitle>
        <VueDatePicker
          v-model="updatedStrategy.backtest_config.end_time"
          :enable-time-picker="false"
          format="yyyy/MM/dd"
          :auto-apply="true"
        ></VueDatePicker>
        <v-list-item-subtitle> End Time </v-list-item-subtitle>
        <v-select
          v-model="updatedStrategy.backtest_config.calendar"
          :items="['XNYS', 'XNAS']"
          label="Calendar"
        ></v-select>
        <v-select
          v-model="updatedStrategy.backtest_config.symbols"
          :items="storedSymbols"
          label="Symbols"
          multiple
        ></v-select>
        <v-select
          v-model="updatedStrategy.backtest_config.benchmark"
          :items="storedSymbols"
          label="Benchmark"
        ></v-select>
      </v-list>
    </template>
    <v-card-actions>
      <v-btn @click="this.update">Update</v-btn>
    </v-card-actions>
  </v-card>
</template>

<script>
import VueDatePicker from "@vuepic/vue-datepicker";
import "@vuepic/vue-datepicker/dist/main.css";
import { getAssets } from "@/modules/finance/backend";
import { isEqual } from "lodash";
import { updateStrategy } from "@/modules/strategies/backend";

export default {
  props: {
    strategy: {
      type: Object,
      required: true,
    },
  },
  components: {
    VueDatePicker,
  },
  data() {
    return {
      storedStrategy: null,
      updatedStrategy: null,
      assets: [],
      storedSymbols: [],
    };
  },
  methods: {
    update: async function () {
      console.log("Update strategy");
      try {
        let strategy = await updateStrategy(
          this.strategy.name,
          this.updatedStrategy
        );
        console.log(strategy);
        this.storedStrategy = strategy;
        this.updatedStrategy = { ...strategy };
      } catch (error) {
        console.log(error);
      }
    },
  },
  computed: {
    strategyChanged: function () {
      // TODO : use this to disable the update button in case the strategy has not changed
      console.log(this.storedStrategy);
      console.log(this.updatedStrategy);
      return isEqual(this.updatedStrategy, this.storedStrategy);
    },
  },
  async created() {
    this.storedStrategy = this.strategy;
    this.updatedStrategy = { ...this.strategy };
    this.assets = await getAssets();
    this.storedSymbols = this.assets.map((asset) => asset.symbol);
    this.symbols = this.storedSymbols;

    console.log(this.updatedStrategy.start);
    if (this.updatedStrategy.start === undefined) {
      console.log("Settings");
      this.updatedStrategy.backtest_config.start =
        this.assets[0].start_ohlc;
    }
    if (this.updatedStrategy.end_time === undefined) {
      this.updatedStrategy.backtest_config.end_time = this.assets[0].end_ohlc;
    }
  },
};
</script>

<style>
</style>
