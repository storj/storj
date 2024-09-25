// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-snackbar
    v-model="doNotificationsExist"
    absolute
    top
    right
    class="custom-snackbar"
  >
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

}
</script>
<style lang="scss" scoped>
.custom-snackbar {
  ::v-deep .v-snack__content {
    margin-right: -9px;
  }
  .v-alert {
    margin: 10px;
  }
  ::v-deep .v-snack__wrapper.theme--dark {
    background-color: transparent;
    color: rgba(255, 255, 255, 0.87);
  }
  ::v-deep .v-sheet.v-snack__wrapper:not(.v-sheet--outlined) {
    box-shadow: none;
  }
  ::v-deep .v-alert__icon.v-icon{
    top: 12px;
  }
}
</style>
