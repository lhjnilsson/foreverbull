<template>
  <v-card v-if="serviceData">
    <template v-slot:title>
      Service Information: {{ serviceData.type }}: {{ serviceData.name }}
    </template>
    <template v-slot:subtitle>
      Image: {{ serviceData.image }}, Status: {{ serviceData.status }}
      <br><br>
        <template v-if="serviceData.status == 'ERROR'">
        <v-icon
            icon="mdi-alert"
            size="18"
            color="error"
            class="me-1 pb-1"
          ></v-icon>
        {{ serviceData.message }}
      </template>
    </template>
    <template v-slot:text>
      <v-list-item :title="serviceData.name" subtitle="Service Name" />
      <v-list-item :title="serviceData.type" subtitle="Service type" />
      <v-list lines="two" v-if="serviceData.type === 'worker'">
        <v-list-subheader inset>Parameters</v-list-subheader>
          <v-list-item
          v-for="parameter in serviceData.parameters"
          :key="parameter.key"
          :title="parameter.key"
          :subtitle="'Type: ' + parameter.type + ', default: ' + parameter.default"
        > </v-list-item>
      </v-list>
    </template>
  </v-card>
</template>

<script>
import { getService } from '@/modules/services/backend'

export default {
  props: {
    name: {
      type: String,
      required: false,
    },
    service: {
      type: Object,
      required: false,
    }
  },
  components: {
  },
  data() {
    return {
      serviceData: null,
    }
  },
  methods: {
  },
  async created() {
    if (this.service) {
      this.serviceData = this.service;
      return;
    }
    this.serviceData = await getService(this.name);
  },
}
</script>

<style>

</style>
