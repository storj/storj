// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        border="start"
        :type="item.type"
        dismissible
        @mouseover="() => onMouseOver(item.id)"
        @mouseleave="() => onMouseLeave(item.id)"
        @input="() => onCloseClick(item.id)"
    >
        <div class="text-h6">{{ item.title }}</div>
        <div>{{ item.message }}</div>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert } from 'vuetify/components';

import { DelayedNotification } from '@/app/types/delayedNotification';
import { useNotificationsStore } from '@/app/store/notificationsStore';

const notificationsStore = useNotificationsStore();

defineProps<{
    item: DelayedNotification;
}>();

function onMouseOver(id: string): void {
    notificationsStore.pauseNotification(id);
}

function onMouseLeave(id: string): void {
    notificationsStore.resumeNotification(id);
}

function onCloseClick(id: string): void {
    notificationsStore.deleteNotification(id);
}
</script>
