<template>
  <v-card>
    <v-toolbar>
      <v-toolbar-title>Instances</v-toolbar-title>
      <v-spacer></v-spacer>
    </v-toolbar>
    <v-table>
      <thead>
      <th>
        ID
      </th>
      <th>
        Role
      </th>
      <th>
        Key
      </th>
      <th>
        Created At
      </th>
      <th>
        Started At
      </th>
      <th>
        Stopped At
      </th>
    </thead>
    <tbody>
      <tr
        v-for="instance in instances"
        :key="instance.id"
      >
        <td>{{ instance.id }}</td>
        <td>{{ instance.role }}</td>
        <td>{{ instance.key }}</td>
        <td>{{ $filters.formatDate(instance.created_at) }}</td>
        <td>{{ $filters.formatDate(instance.started_at) }}</td>
        <td>{{ $filters.formatDate(instance.stopped_at) }}</td>
      </tr>
    </tbody>
    </v-table>
  </v-card>
</template>

<script>
import { getInstances } from '@/modules/services/backend';

export default {
  props: {
    service: {
      type: Object,
      required: false,
    },
    role: {
      type: String,
      required: false,
    },
    roleKey: {
      type: String,
      required: false,
    },
  },
  data() {
    return {
      instances: [],
    };
  },
  async created() {
    this.instances = await getInstances(this.service.name, this.role, this.roleKey);
  }
}
</script>

<style>

</style>
