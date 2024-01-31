<template>
  <v-app>
    <app-bar />
    <nav-bar />

    <v-main>
      <router-view />
    </v-main>
  </v-app>
</template>

<script setup>
  import AppBar from './AppBar.vue'
  import NavBar from './NavBar.vue'
</script>
