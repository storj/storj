// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <router-view />
    <Notifications />
</template>

<script setup lang="ts">
import { onMounted } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';

import Notifications from '@poc/layouts/default/Notifications.vue';

const configStore = useConfigStore();

/**
 * Lifecycle hook after initial render.
 * Sets up variables from meta tags from config such satellite name, etc.
 */
onMounted(async (): Promise<void> => {
    await configStore.getConfig();
});
</script>
