// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="appStore.state.settings">
        <router-view />
        <notifications />
    </template>

    <FullScreenLoader :model-value="!appStore.state.settings" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue';

import { useAppStore } from '@/store/app';
import { useNotify } from '@/composables/useNotify';
import { DARK_THEME_QUERY, useThemeStore } from '@/store/theme';

import Notifications from '@/layouts/default/Notifications.vue';
import FullScreenLoader from '@/components/FullScreenLoader.vue';

const appStore = useAppStore();
const themeStore = useThemeStore();
const notify = useNotify();

const darkThemeMediaQuery = window.matchMedia(DARK_THEME_QUERY);

function onThemeChange(e: MediaQueryListEvent) {
    themeStore.setThemeLightness(!e.matches);
}

watch(() => themeStore.state.name, (theme) => {
    if (theme === 'auto') {
        darkThemeMediaQuery.addEventListener('change', onThemeChange);
        return;
    }
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
}, { immediate: true });

onMounted(async () => {
    try {
        await Promise.all([
            appStore.getSettings(),
            appStore.getPlacements(),
            appStore.getProducts(),
        ]);
    } catch (error) {
        notify.error(`Failed to initialise app. ${error.message}`);
    }
});

onBeforeUnmount(() => {
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
});
</script>
