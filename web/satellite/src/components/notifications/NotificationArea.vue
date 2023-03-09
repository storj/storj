// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="doNotificationsExist" class="notification-container">
        <NotificationItem
            v-for="notification in notifications"
            :key="notification.id"
            :notification="notification"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { DelayedNotification } from '@/types/DelayedNotification';

import NotificationItem from '@/components/notifications/NotificationItem.vue';

// @vue/component
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
