<template>
  <v-navigation-drawer permanent>
    <v-list
      density="compact"
      nav
    >
      <router-link :to="{name: 'Dashboard'}" active-class="active">
        <v-list-item prepend-icon="mdi-view-dashboard" title="Dashboard" value="dashboard"></v-list-item>
      </router-link>
      <router-link :to="{name: 'Finance'}" active-class="active">
        <v-list-item prepend-icon="mdi-chart-box-plus-outline" title="Finance" value="finance"></v-list-item>
      </router-link>
      <router-link :to="{name: 'Services'}" active-class="active">
        <v-list-item prepend-icon="mdi-lan" title="Services" value="services"></v-list-item>
      </router-link>
      <router-link :to="{name: 'Backtests'}" active-class="active">
        <v-list-item prepend-icon="mdi-chart-bubble" title="Backtests" value="backtests"></v-list-item>
      </router-link>
      <router-link :to="{name: 'Strategies'}" active-class="active">
        <v-list-item prepend-icon="mdi-chart-multiple" title="Strategies" value="strategies"></v-list-item>
      </router-link>
    </v-list>
  </v-navigation-drawer>
</template>

<script setup>
  //
</script>

<style scoped>
.v-list-item {
  cursor: pointer;
  font-family: 'Roboto', sans-serif;
  font-size: 18px;
}
</style>
