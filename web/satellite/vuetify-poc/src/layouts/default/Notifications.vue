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
            :title="title(item.type)"
            :text="item.messageNode ? '' : item.message"
            :type="getType(item.type)"
            rounded="lg"
            class="my-2"
            border
            @mouseover="() => onMouseOver(item.id)"
            @mouseleave="() => onMouseLeave(item.id)"
            @click:close="() => onCloseClick(item.id)"
        >
            <template #default>
                <!-- eslint-disable-next-line vue/no-v-html -->
                <div v-if="item.messageNode" v-html="item.messageNode" />
            </template>
        </v-alert>
    </v-snackbar>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VAlert, VSnackbar } from 'vuetify/components';

import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { DelayedNotification, NOTIFICATION_TYPES } from '@/types/DelayedNotification';

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
 * Returns notification title based on type.
 * @param itemType
 */
function title(itemType: string): string {
    const type = getType(itemType);
    const [firstLetter, ...rest] = type;

    return `${firstLetter.toUpperCase()}${rest.join('')}`;
}

/**
 * Returns notification type.
 * @param itemType
 */
function getType(itemType: string): string {
    switch (itemType) {
    case NOTIFICATION_TYPES.SUCCESS:
        return 'success';
    case NOTIFICATION_TYPES.ERROR:
        return 'error';
    case NOTIFICATION_TYPES.WARNING:
        return 'warning';
    default:
        return 'info';
    }
}

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