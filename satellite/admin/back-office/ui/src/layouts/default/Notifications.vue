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
        <v-alert
            v-for="item in notifications"
            :key="item.id"
            closable
            variant="elevated"
            :title="item.title || item.type"
            :type="item.alertType"
            rounded="lg"
            class="my-2"
            border
            @mouseover="() => onMouseOver(item.id)"
            @mouseleave="() => onMouseLeave(item.id)"
            @click:close="() => onCloseClick(item.id)"
        >
            <template #default>
                <component :is="item.messageNode" />
            </template>
        </v-alert>
    </v-snackbar>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VAlert, VSnackbar } from 'vuetify/components';

import { useNotificationsStore } from '@/store/notifications';
import { DelayedNotification } from '@/types/notifications';

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

/**
 * Forces notification to stay on page on mouse over it.
 */
function onMouseOver(id: string): void {
    notificationsStore.pauseNotification(id);
}

/**
 * Resume notification flow when mouse leaves notification.
 */
function onMouseLeave(id: string): void {
    notificationsStore.resumeNotification(id);
}

/**
 * Removes notification when the close button is clicked.
 */
function onCloseClick(id: string): void {
    notificationsStore.deleteNotification(id);
}
</script>
