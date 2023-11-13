// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="objects-area">
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

const router = useRouter();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();

/**
 * Lifecycle hook after initial render.
 * Redirects if flow is disabled.
 */
onMounted(async (): Promise<void> => {
    if (configStore.state.config.fileBrowserFlowDisabled) {
        analyticsStore.pageVisit(RouteConfig.ProjectDashboard.path);
        await router.push(RouteConfig.ProjectDashboard.path);
    }
});
</script>

<style scoped lang="scss">
    .objects-area {
        padding-bottom: 55px;
    }
</style>
