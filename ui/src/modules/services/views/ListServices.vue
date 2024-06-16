<template>
  <v-container>
    <v-card>
      <v-toolbar>
      <v-toolbar-title>Services</v-toolbar-title>
      <v-spacer></v-spacer>
      <add-service-dialog @added="serviceAdded" @error="errorAdded"></add-service-dialog>
    </v-toolbar>
    <v-alert v-if="addedStatusType.length > 0" closable :text="addedStatusText" :type="addedStatusType" variant="tonal"></v-alert>
    <v-table hover>
      <thead>
        <tr>
          <th class="text-left">
            Name
          </th>
          <th class="text-left">
            Status
          </th>
          <th class="text-left">
            Service Type
          </th>
          <th class="text-left">
            Created
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="item in  services"
          :key="item.name"
          @click="openService(item)"
        >
          <td>{{ item.name }}</td>
          <td>{{ item.status }}</td>
          <td>{{ item.type }}</td>
          <td>{{ $filters.formatDate(item.created_at) }}</td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
  </v-container>
</template>

<script>
import AddServiceDialog from "@/modules/services/components/AddServiceDialog.vue";
import { getServices } from "@/modules/services/backend";

export default {
  components: {
    AddServiceDialog,
  },
  data() {
    return {
      services: [],
      addedStatusType: "",
      addedStatusText: null,
    };
  },
  methods: {
    async serviceAdded(service) {
      this.addedStatusType = "success";
      this.addedStatusText = `Service ${service.name} added`;
      this.services = await getServices();
    },
    async errorAdded(error) {
      this.addedStatusType = "error";
      this.addedStatusText = `Error adding service: ${error.message}`;
    },
    openService(service) {
      this.$router.push({ name: "Service", params: { name: service.name } });
    }
  },
  async mounted() {
    this.services = await getServices();
  },
}
</script>

<style>
</style>
