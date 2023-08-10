// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <slot v-bind="sessionTimeout" />
    <InactivityModal
        v-if="sessionTimeout.inactivityModalShown.value"
        :on-continue="() => sessionTimeout.refreshSession(true)"
        :on-logout="sessionTimeout.handleInactive"
        :on-close="() => sessionTimeout.inactivityModalShown.value = false"
        :initial-seconds="INACTIVITY_MODAL_DURATION / 1000"
    />
    <SessionExpiredModal v-if="sessionTimeout.sessionExpiredModalShown.value" :on-redirect="redirectToLogin" />
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';

import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useSessionTimeout, INACTIVITY_MODAL_DURATION } from '@/composables/useSessionTimeout';
import { RouteConfig } from '@/types/router';

import InactivityModal from '@/components/modals/InactivityModal.vue';
import SessionExpiredModal from '@/components/modals/SessionExpiredModal.vue';

const analyticsStore = useAnalyticsStore();

const sessionTimeout = useSessionTimeout();
const router = useRouter();

/**
 * Redirects to log in screen.
 */
function redirectToLogin(): void {
    analyticsStore.pageVisit(RouteConfig.Login.path);
    router.push(RouteConfig.Login.path);

    sessionTimeout.sessionExpiredModalShown.value = false;
}
</script>
