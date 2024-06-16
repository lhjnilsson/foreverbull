<template>
  <v-container fluid fill-height>
    <v-card v-if="this.workerService">
      <v-card-title> Create Execution </v-card-title>

      <v-layout>
        <table style="width: 100%">
          <thead>
            <th>Num</th>
            <th v-for="param in this.workerService.parameters" :key="param.key">
              {{ param.key }}
            </th>
          </thead>
          <tbody>
            <tr v-for="(execution, index) in this.executions" :key="execution">
              <td>{{ index }}</td>
              <td v-for="param in execution.parameters" :key="param.key">
                {{ param.value }}
              </td>
            </tr>
          </tbody>
        </table>
      </v-layout>

      <v-card-actions>
        <v-text-field
          variant="underlined"
          v-for="param in this.parameters"
          :label="param.key"
          :key="param.key"
          v-model="param.value"
        ></v-text-field>
        <v-btn color="blue-darken-1" variant="text" @click="addExecution"
          >Add</v-btn
        >
        <v-spacer></v-spacer>
        <v-btn color="blue-darken-1" variant="text" @click="createExecution">
          Create
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-container>
</template>

<script>
import { getService } from "@/modules/services/backend";
import { createSession } from "@/modules/backtests/backend";
export default {
  props: {
    backtest: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      name: null,
      value: null,
      workerService: null,
      defaultParameters: [],
      parameters: [],
      executions: [],
    };
  },
  async mounted() {
    this.workerService = await getService(this.backtest.worker_service);
    for (let i = 0; i < this.workerService.parameters.length; i++) {
      this.defaultParameters.push({
        key: this.workerService.parameters[i].key,
        value: this.workerService.parameters[i].default,
      });
    }

    this.parameters = [];
    for (let i = 0; i < this.defaultParameters.length; i++) {
      this.parameters.push({
        key: this.defaultParameters[i].key,
        value: this.defaultParameters[i].value,
      });
    }
  },
  methods: {
    addExecution() {
      let parameters = [];
      for (let i = 0; i < this.parameters.length; i++) {
        parameters.push({
          key: this.parameters[i].key,
          value: this.parameters[i].value,
        });
      }
      this.executions.push({
        parameters: parameters,
      });

      this.parameters = [];
      for (let i = 0; i < this.defaultParameters.length; i++) {
        this.parameters.push({
          key: this.defaultParameters[i].key,
          value: this.defaultParameters[i].value,
        });
      }
    },
    async createExecution() {
      await createSession(this.backtest.name, this.executions);
      this.$emit("sessionCreated");
    },
  },
};
</script>

<style>
</style>
