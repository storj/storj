<template>
    <v-snackbar
      v-model="doNotificationsExist"
      absolute
      top
      right
      class="custom-snackbar"
    >
      <v-alert
        text
        title="Alert title"
        type="success"
        class="custom-alert"
      >
        This is a notification
      </v-alert>
    </v-snackbar>
  </template>

<script lang="ts">
import { VSnackbar } from 'vuetify/lib';
// import NotificationItem from './NotificationItem.vue';
import { Component, Vue } from 'vue-property-decorator';
// import { DelayedNotification } from '@/app/types/delayedNotification';
import { VAlert } from 'vuetify/lib';

@Component({
    components: {
       VSnackbar,
       VAlert
    //    NotificationItem
    },
})

export default class Notifications extends Vue {

    // public get notifications(): DelayedNotification[]{
    //     return this.$store.state.notification.notificationQueue;
    // }

    public doNotificationsExist(): boolean {
        // return this.notifications.length > 0;
        return true;
    }
}
</script>
