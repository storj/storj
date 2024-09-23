<template>
  <v-alert border="left" :type="item.type.toLowerCase()" dismissible>
    <div class="text-h6">{{ item.title }}</div>
    <div>{{ item.message }}</div>
  </v-alert>
</template>

<script lang="ts">
import { Component, Vue, Prop } from "vue-property-decorator";
import { VAlert } from "vuetify/lib";
import { DelayedNotification } from "@/app/types/delayedNotification";

@Component({
  components: {
    VAlert,
  },
})
export default class NotificationItem extends Vue {
  @Prop({ required: true }) readonly item!: DelayedNotification;

  created() {
    console.log("Notification Item:", this.item);
  }
}
</script>
