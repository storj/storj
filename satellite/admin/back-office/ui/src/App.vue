// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="appStore.state.settings">
        <router-view />
        <notifications />
    </template>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';

import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';

import Notifications from '@/layouts/default/Notifications.vue';

const appStore = useAppStore();
const notify = useNotificationsStore();

onMounted(async () => {
    try {
        await appStore.getPlacements();
    } catch (error) {
        notify.notifyError(`Failed to get placements. ${error.message}`);
    }

    try {
        await appStore.getSettings();
    } catch (error) {
        notify.notifyError(`Failed to get settings. ${error.message}`);
    }
});
</script>
