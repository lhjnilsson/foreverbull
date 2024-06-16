<template>
  <v-card>
    <v-toolbar>
      <v-toolbar-title>Sessions</v-toolbar-title>
      <v-spacer></v-spacer>
    </v-toolbar>
    <v-container fluid fill-height>
      <v-table style="width: 100%" hover>
        <thead>
          <tr>
            <th class="text-left">Created At</th>
            <th class="text-left">Started At</th>
            <th class="text-left">Finished At</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="session in sessions" :key="session.id" @click="$emit('session', session)">
            <td>{{ $filters.formatDate(session.created_at) }}</td>
            <td>{{ $filters.formatDate(session.started_at) }}</td>
            <td>{{ $filters.formatDate(session.finished_at) }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-container>
  </v-card>
</template>

<script>
export default {
  props: {
    sessions: {
      type: Array,
      required: true,
    },
  },
};
</script>

<style>
</style>
