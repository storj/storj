// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-container" v-if="doNotificationsExist">
        <NotificationItem
            v-for="notification in notifications"
            :notification="notification"
            :key="notification.id"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NotificationItem from '@/components/notifications/NotificationItem.vue';

import { DelayedNotification } from '@/types/DelayedNotification';

@Component({
    components: {
        NotificationItem,
    },
})
export default class NotificationArea extends Vue {
    /**
     * Returns all notification queue from store.
     */
    public get notifications(): DelayedNotification[] {
        return this.$store.state.notificationsModule.notificationQueue;
    }

    /**
     * Indicates if any notifications are in queue.
     */
    public get doNotificationsExist(): boolean {
        return this.notifications.length > 0;
    }
}
</script>

<style scoped lang="scss">
    .notification-container {
        width: 417px;
        background-color: transparent;
        display: flex;
        flex-direction: column;
        position: fixed;
        top: 114px;
        right: 17px;
        align-items: flex-end;
        justify-content: space-between;
        border-radius: 12px;
        z-index: 9999;
        overflow: hidden;
    }
</style>
