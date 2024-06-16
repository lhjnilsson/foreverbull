<template>
  <v-data-table
    :headers="headers"
    :items="orders"
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
    orders: {
      type: Array,
      required: true,
    }
  },
  data() {
    return {
      headers: [
        { title: "Symbol", key: "symbol", },
        { title: "Amount", key: "amount", } ,
        { title: "Limit Price", key: "limit_price"},
        { title: "Stop Price", key: "stop_price"},
        { title: "Status", key: "status"},
        { title: "Created", key: "created_at"},
      ],
      itemsPerPage: 20,
    }
  },
}
</script>

<style>

</style>
