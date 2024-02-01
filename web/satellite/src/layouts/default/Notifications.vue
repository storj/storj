// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-snackbar
        v-model="doNotificationsExist"
        position="fixed"
        location="top right"
        z-index="99999"
        variant="text"
    >
        <notification-item
            v-for="item in notifications"
            :key="item.id"
            :item="item"
        />
    </v-snackbar>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VSnackbar } from 'vuetify/components';

import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { DelayedNotification } from '@/types/DelayedNotification';

import NotificationItem from '@/components/NotificationItem.vue';

const notificationsStore = useNotificationsStore();

/**
 * Indicates if any notifications are in queue.
 */
const doNotificationsExist = computed((): boolean => {
    return notifications.value.length > 0;
});

/**
 * Returns all notification queue from store.
 */
const notifications = computed((): DelayedNotification[] => {
    return notificationsStore.state.notificationQueue as DelayedNotification[];
});
</script>
