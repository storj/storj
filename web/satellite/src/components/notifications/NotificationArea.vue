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

<script setup lang="ts">
import { computed } from 'vue';

import { DelayedNotification } from '@/types/DelayedNotification';
import { useNotificationsStore } from '@/store/modules/notificationsStore';

import NotificationItem from '@/components/notifications/NotificationItem.vue';

const notificationsStore = useNotificationsStore();

/**
 * Returns all notification queue from store.
 */
const notifications = computed((): DelayedNotification[] => {
    return notificationsStore.state.notificationQueue as DelayedNotification[];
});

/**
 * Indicates if any notifications are in queue.
 */
const doNotificationsExist = computed((): boolean => {
    return notifications.value.length > 0;
});
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

        @media screen and (width <= 450px) {
            width: unset;
            left: 17px;
        }
    }
</style>
