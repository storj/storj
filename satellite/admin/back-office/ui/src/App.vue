// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <router-view />
    <notifications />
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
});
</script>
