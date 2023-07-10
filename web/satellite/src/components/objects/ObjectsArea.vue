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
import { AnalyticsHttpApi } from '@/api/analytics';
import { useConfigStore } from '@/store/modules/configStore';

const router = useRouter();
const configStore = useConfigStore();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Lifecycle hook after initial render.
 * Redirects if flow is disabled.
 */
onMounted(async (): Promise<void> => {
    if (configStore.state.config.fileBrowserFlowDisabled) {
        analytics.pageVisit(RouteConfig.ProjectDashboard.path);
        await router.push(RouteConfig.ProjectDashboard.path);
    }
});
</script>

<style scoped lang="scss">
    .objects-area {
        padding-bottom: 55px;
    }
</style>
