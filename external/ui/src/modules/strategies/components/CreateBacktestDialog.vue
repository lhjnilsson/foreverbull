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
          :disabled="disabled"
        >
          New Execution
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="text-h5">New backtest execution</span>
        </v-card-title>
        <v-card-text>
          <v-container>
            <v-form>
              <v-row>
                <v-col cols="12" md="12">
                  <v-text-field
                  v-for="parameter in worker.parameters"
                  :key="parameter.key"
                  v-model="parameters[parameter.key]"
                  :label="parameter.key"></v-text-field>
                </v-col>
                <v-col cols="12" md="12">
                  <v-select
                    v-model="clientLogLevel"
                    :items="['DEBUG', 'INFO', 'WARNING', 'ERROR']"
                    label="Client Log Level"
                  ></v-select>
                </v-col>
                <v-col cols="12" md="12">
                  <v-checkbox
                    v-model="removeWhenFinished"
                    label="Remove When Finished"
                  ></v-checkbox>
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
import { getService } from '@/modules/services/backend'
import { createBacktest } from '@/modules/strategies/backend'

export default {
  props: {
    strategy: {
      type: Object,
      required: true,
    },
    disabled: {
      type: Boolean,
      required: false,
      default: false,
    }
  },
  data: () => ({
    worker: null,
    dialog: false,
    parameters: {},
    clientLogLevel: 'INFO',
    removeWhenFinished: true,
  }),
  methods: {
    async createBacktest() {
      console.log("create backtest")
      try {
        await createBacktest(this.strategy.name, this.parameters, this.clientLogLevel, this.removeWhenFinished)
        this.dialog = false
      } catch (error) {
        console.log("ERR: ", error.message)
      }
      this.dialog = false
    }
  },
  async created() {
    this.worker = await getService(this.strategy.worker)
    for (const parameter of this.worker.parameters) {
      this.parameters[parameter.key] = parameter.default
    }
    console.log("Parameters: ", this.parameters)
  },
}
</script>

<style>

</style>
