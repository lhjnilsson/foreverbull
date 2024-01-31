<template>
  <v-card v-if="image">
    <v-card-title>
      <v-icon>mdi-docker</v-icon>
      <span class="ml-2">Image information</span>
    </v-card-title>
    <v-card-text>
      <v-row>
        <v-col>
          <v-list>
            <v-list-item>
              <v-list-item-content>
                <v-list-item-title>Id</v-list-item-title>
                <v-list-item-subtitle>{{ image.Id }}</v-list-item-subtitle>
              </v-list-item-content>
            </v-list-item>
            <v-list-item>
              <v-list-item-content>
                <v-list-item-title>Tags</v-list-item-title>
                <v-list-subtitle v-for="tag in image.RepoTags" :key="tag">{{
                  tag
                }}</v-list-subtitle>
              </v-list-item-content>
            </v-list-item>
          </v-list>
        </v-col>
      </v-row>
    </v-card-text>
    <v-card-actions>
      <v-btn @click="updateImage"> Update Image </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script>
import { updateImage } from "../backend";

export default {
  props: {
    image: Object,
    service: Object,
  },
  data() {
    return {};
  },
  methods: {
    async updateImage() {
      updateImage(this.service.name);
    },
  },
  mounted() {
    console.log("DockerImageCard mounted");
    console.log(this.image);
  },
  async created() {},
};
</script>

<style>
</style>
