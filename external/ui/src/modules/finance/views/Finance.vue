<template>
  <v-container>
    <v-row align="center">
      <v-col>
        <v-text-field
          label="Symbol to add"
          hide-details="auto"
          hint="eg 'AAPL' for Apple."
          v-model="symbolToAdd"
          :error-messages="addAssetError"
          :disabled="disableIngestAsset"
        >
          <template v-slot:append>
            <v-btn flat @click="addAsset">
              <v-icon>mdi-plus</v-icon>
            </v-btn>
          </template>
        </v-text-field>
      </v-col>
      <v-col>
        <VueDatePicker
          v-model="selectedDateFrom"
          :enable-time-picker="false"
          :auto-apply="true"
          placeholder="OHLC From"
          format="yyyy/MM/dd"
        ></VueDatePicker>
      </v-col>
      <v-col>
        <VueDatePicker
          v-model="selectedDateTo"
          :enable-time-picker="false"
          :auto-apply="true"
          placeholder="OHLC To"
          format="yyyy/MM/dd"
        ></VueDatePicker>
      </v-col>
      <v-col align="center">
        <v-btn @click="ingestOHLC" flat :disabled="!ohlcDatesUpdated || disableUpdateOHLC">
          <v-icon>{{  disableUpdateOHLC ? 'mdi-progress-download' : 'mdi-download' }}</v-icon>
        </v-btn>
      </v-col>
    </v-row>
    <v-alert v-if="ingestedStatusType.length > 0" closable :text="ingestedStatusText" :type="ingestedStatusType" variant="tonal"></v-alert>
    <assets-table :assets="assets"></assets-table>
  </v-container>
</template>

<script>
import AssetsTable from "@/modules/finance/components/AssetsTable.vue";
import { getAssets, ingestAsset, ingestOHLC } from "@/modules/finance/backend";
import VueDatePicker from "@vuepic/vue-datepicker";
import "@vuepic/vue-datepicker/dist/main.css";

export default {
  components: {
    AssetsTable,
    VueDatePicker,
  },
  data: function () {
    return {
      assets: [],
      ingestedDateFrom: null,
      selectedDateFrom: null,
      ingestedDateTo: null,
      selectedDateTo: null,
      symbolToAdd: "",
      addAssetError: "",
      disableUpdateOHLC: false,
      disableIngestAsset: false,

      ingestedStatusType: "",
      ingestedStatusText: "",
    };
  },
  async created() {
    this.assets = await getAssets();
    // Get the greatest start_ohlc date
    this.ingestedDateFrom = this.assets.reduce(
      (min, p) => (p.start_ohlc > min ? p.start_ohlc : min),
      this.assets[0].start_ohlc
    );
    // Get the smallest end_ohlc date
    this.ingestedDateTo = this.assets.reduce(
      (max, p) => (p.end_ohlc < max ? p.end_ohlc : max),
      this.assets[0].end_ohlc
    );
    this.selectedDateFrom = this.ingestedDateFrom;
    this.selectedDateTo = this.ingestedDateTo;
  },
  methods: {
    async addAsset() {
      this.disableIngestAsset = true;
      try {
        ingestAsset(this.symbolToAdd)
          .then(async () => {
            this.assets = await getAssets();
            this.ingestedStatusType = "success";
            this.ingestedStatusText = `Asset ${this.symbolToAdd} added successfully`;
            this.addAssetError = "";
            this.symbolToAdd = "";
          })
          .catch((e) => {
            console.log(e)
            this.addAssetError = e.response.data.message;
            this.ingestedStatusType = "error";
            this.ingestedStatusText = `Error adding asset ${this.symbolToAdd}: ${this.addAssetError}`;
          });
      }
      finally {
        this.disableIngestAsset = false;
      }
    },
    async ingestOHLC() {
      this.disableUpdateOHLC = true;
      var symbols = this.assets.map(asset => asset.symbol)
      ingestOHLC(symbols, this.selectedDateFrom, this.selectedDateTo)
        .then(async () => {
          this.ingestedDateFrom = this.selectedDateFrom;
          this.ingestedDateTo = this.selectedDateTo;
          this.assets = await getAssets();
          this.ingestedStatusType = "success";
          this.ingestedStatusText = `OHLC data updated successfully`;
        })
        .catch((e) => {
          console.log(e)
          this.addAssetError = e.response.data.message;
          this.ingestedStatusType = "error";
          this.ingestedStatusText = `Error updating OHLC data: ${this.addAssetError}`;
        })
        .finally(() => {
          this.disableUpdateOHLC = false;
        })
    },
  },
  computed: {
    ohlcDatesUpdated() {
      return (
        this.selectedDateFrom !== this.ingestedDateFrom ||
        this.selectedDateTo !== this.ingestedDateTo
      );
    },
  },
};
</script>

<style>
</style>
