// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-banner
        severity="info"
        :dashboard-ref="dashboardRef"
        :on-close="onCloseClick"
    >
        <template #text>
            <p class="medium">
                You can now update your session timeout from your
                <span class="link" @click.stop.self="redirectToSettingsPage">account settings</span>
            </p>
        </template>
    </v-banner>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';

import { RouteConfig } from '@/router';
import { useAppStore } from '@/store/modules/appStore';

import VBanner from '@/components/common/VBanner.vue';

const appStore = useAppStore();
const router = useRouter();
const route = useRoute();

const props = defineProps<{
    dashboardRef: HTMLElement
}>();

/**
 * Redirects to settings page.
 */
function redirectToSettingsPage(): void {
    onCloseClick();

    if (route.path.includes(RouteConfig.AllProjectsDashboard.path)) {
        router.push(RouteConfig.AccountSettings.with(RouteConfig.Settings2).path);
        return;
    }

    router.push(RouteConfig.Account.with(RouteConfig.Settings).path);
}

/**
 * Closes notification.
 */
function onCloseClick(): void {
    appStore.closeUpdateSessionTimeoutBanner();
}
</script>
