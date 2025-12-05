// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        border="left"
        :type="item.type.toLowerCase()"
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
import { VAlert } from 'vuetify/lib';

import { DelayedNotification } from '@/app/types/delayedNotification';
import { useStore } from '@/app/utils/composables';

const store = useStore();

defineProps<{
    item: DelayedNotification;
}>();

function onMouseOver(id: string): void {
    store.dispatch('notification/pause', id);
}

function onMouseLeave(id: string): void {
    store.dispatch('notification/resume', id);
}

function onCloseClick(id: string): void {
    store.dispatch('notification/delete', id);
}
</script>
