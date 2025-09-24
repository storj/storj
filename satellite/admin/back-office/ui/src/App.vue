// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        v-if="!appStore.state.settings"
        class="d-flex justify-center align-center align-items-center"
        style="height: 100vh;"
    >
        <v-skeleton-loader
            class="mx-auto"
            width="300"
            height="200"
            type="card"
        />
    </div>
    <template v-else>
        <router-view />
        <notifications />
    </template>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue';
import { VSkeletonLoader } from 'vuetify/components';

import { useAppStore } from '@/store/app';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';
import { DARK_THEME_QUERY, useThemeStore } from '@/store/theme';

import Notifications from '@/layouts/default/Notifications.vue';

const appStore = useAppStore();
const themeStore = useThemeStore();
const usersStore = useUsersStore();
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
            usersStore.getAccountFreezeTypes(),appStore.getSettings(),
            appStore.getPlacements(),
        ]);
    } catch (error) {
        notify.error(`Failed to initialise app. ${error.message}`);
    }
});

onBeforeUnmount(() => {
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
});
</script>
