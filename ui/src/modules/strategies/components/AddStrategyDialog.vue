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
          Add Strategy
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="text-h5">Add new Strategy</span>
        </v-card-title>
        <v-card-text>
          <v-container>
            <v-form>
              <v-row>
                <v-col cols="12" md="12">
                  <v-text-field
                    label="Name"
                    v-model="name"
                  ></v-text-field>
                  <v-select
                    label="Backtest"
                    v-model="backtest"
                    :items="backtests"
                    item-title="name"
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
            @click="addStrategy"
          >
            Create
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-row>
</template>

<script>
import { addStrategy } from '@/modules/strategies/backend'
import { getBacktests } from '@/modules/backtests/backend'

export default {
  components: {
  },
  data: () => ({
    dialog: false,
    backtests: [],
    name: null,
    backtest: null,
  }),
  methods: {
    async addStrategy() {
      try {
        let strategy = await addStrategy(this.name, this.backtest.name)
        this.$emit('added', strategy)
      } catch (error) {
        this.$emit('error', error)
      }
      this.dialog = false
    }
  },
  async mounted() {
    try {
      this.backtests = await getBacktests()
      if (this.backtests.length > 0) {
        this.backtest = this.backtests[0]
      }
    } catch (error) {
      this.$emit('error', error)
    }
  }
}
</script>

<style>

</style>
