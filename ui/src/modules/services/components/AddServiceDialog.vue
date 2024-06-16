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
          Add Service
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="text-h5">Add new Service</span>
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
                    v-model="image"
                    :items="images"
                    label="Image"
                  ></v-select>
                  <v-text-field
                    :disabled="image !== 'Custom'"
                    label="Custom Image"
                    v-model="customImage"
                  ></v-text-field>
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
            @click="addService"
          >
            Create
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-row>
</template>

<script>
import { addService } from '@/modules/services/backend'

export default {
  data: () => ({
    dialog: false,
    name: null,
    image: "Custom",
    customImage: "",
    images: [
      "Custom",
      "quay.io/foreverbull/zipline",
      "quay.io/foreverbull/client-random-example",
      "quay.io/foreverbull/client-ema-example",
    ]
  }),
  methods: {
    async addService() {
      try {
        let image = this.image
        if (this.image == "Custom") {
          image = this.customImage
        }
        let service = await addService(this.name, image)
        this.$emit('added', service)
      } catch (error) {
        this.$emit('error', error)
      }
      this.dialog = false
    }
  },
}
</script>

<style>

</style>
