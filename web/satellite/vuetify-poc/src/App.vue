// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <ErrorPage v-if="isErrorPageShown" />
    <router-view v-else />
    <Notifications />
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@poc/store/appStore';
import { APIError } from '@/utils/error';

import Notifications from '@poc/layouts/default/Notifications.vue';
import ErrorPage from '@poc/components/ErrorPage.vue';

const appStore = useAppStore();
const configStore = useConfigStore();

/**
 * Indicates whether an error page should be shown in place of the router view.
 */
const isErrorPageShown = computed<boolean>((): boolean => {
    return appStore.state.error.visible;
});

/**
 * Lifecycle hook after initial render.
 * Sets up variables from meta tags from config such satellite name, etc.
 */
onMounted(async (): Promise<void> => {
    try {
        await configStore.getConfig();
    } catch (error) {
        appStore.setErrorPage((error as APIError).status ?? 500, true);
    }
});
</script>
