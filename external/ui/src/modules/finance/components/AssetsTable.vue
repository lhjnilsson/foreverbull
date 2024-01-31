<template>
  <v-data-table
    :headers="headers"
    :items="assets"
    :items-per-page="itemsPerPage"
    class="elevation-1"></v-data-table>
</template>

<script>
import { VDataTable } from 'vuetify/labs/VDataTable'

export default {
  components: {
    VDataTable
  },
  props: {
    assets: {
      type: Array,
      required: false,
    }
  },
  data: function() {
    return {
      headers: [
        { title: "Symbol", key: "symbol", },
        { title: "Title", key: "title"},
        { title: "Name", key: "name", },
        { title: "Asset Type", key: "type"},
      ],
      itemsPerPage: 20,
    }
  },
}
</script>

<style>

</style>
