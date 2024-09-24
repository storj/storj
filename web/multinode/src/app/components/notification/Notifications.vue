<template>
  <v-snackbar v-model="doNotificationsExist" absolute top right class="custom-snackbar" timeout=5000000>
    <NotificationItem
            v-for="item in notifications"
            :key="item.id"
            :item="item"
     />
  </v-snackbar>
</template>

<script lang="ts">
import { VSnackbar } from "vuetify/lib";
import NotificationItem from "./NotificationItem.vue";
import { Component, Vue, Watch } from "vue-property-decorator";
import { DelayedNotification } from "@/app/types/delayedNotification";

@Component({
  components: {
    VSnackbar,
    NotificationItem,
  },
})
export default class Notifications extends Vue {
  public doNotificationsExist: boolean = false;

  public get notifications(): DelayedNotification[] {
    return this.$store.state.notification.notificationQueue;
  }

  get hasNotifications(): boolean {
    return this.notifications.length > 0;
  }

  @Watch("hasNotifications", { immediate: true })
  onNotificationsChange(newValue: boolean) {
    this.doNotificationsExist = newValue;
  }

  // Optional: If you want to log changes to the console
  @Watch("notifications", { deep: true })
  onNotificationsArrayChange(newValue: DelayedNotification[]) {
    console.log("Notifications changed:", newValue);
  }
}
</script>
<style lang="scss" scoped>
.custom-snackbar {
 ::v-deep .v-snack__content {
  margin-right: -13px;
 }
}
</style>