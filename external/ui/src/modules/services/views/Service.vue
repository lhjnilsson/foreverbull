<template>
  <v-container v-if="service">
    <v-row>
      <v-col>
        <docker-image-card :image=image :service=service />
      </v-col>
     <v-col>
        <service-card :service=service />
      </v-col>
    </v-row>
    <v-row>
      <v-col>
        <instances-table :service=service />
      </v-col>
    </v-row>
  </v-container>
  <v-container v-else>
    <v-row>
      <v-col>
        <v-progress-circular indeterminate size="64"></v-progress-circular>
      </v-col>
    </v-row>
  </v-container>
</template>

<script>
import DockerImageCard from '@/modules/services/components/DockerImageCard.vue';
import ServiceCard from '@/modules/services/components/ServiceCard.vue';
import InstancesTable from '@/modules/services/components/InstancesTable.vue';

import { getService, imageInfo } from '@/modules/services/backend';

export default {
  components: {
    DockerImageCard,
    ServiceCard,
    InstancesTable
  },
  data() {
    return {
      service: null,
      image: null,
    }
  },
  async created() {
    this.service = await getService(this.$route.params.name);
    this.image = await imageInfo(this.$route.params.name);
  }
}
</script>

<style>

</style>
