/**
 * main.js
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Components
import App from "./App.vue";

// Composables
import { createApp } from "vue";

// Plugins
import { registerPlugins } from "@/plugins";

// Third party
import VueApexCharts from "vue3-apexcharts";
import moment from "moment";

const app = createApp(App);

app.config.globalProperties.$filters = {
  formatDate(value) {
    if (value) {
      return moment(String(value)).format("DD/MM/YYYY hh:mm");
    }
  },
};

registerPlugins(app);

app.mount("#app");
app.use(VueApexCharts);
