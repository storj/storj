// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="objects-area">
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useRouter } from '@/utils/hooks';

const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Lifecycle hook after initial render.
 * Redirects if flow is disabled.
 */
onMounted(async (): Promise<void> => {
    const isFileBrowserFlowDisabled = MetaUtils.getMetaContent('file-browser-flow-disabled');
    if (isFileBrowserFlowDisabled === 'true') {
        analytics.pageVisit(RouteConfig.ProjectDashboard.path);
        await router.push(RouteConfig.ProjectDashboard.path);
    }
});
</script>

<style scoped lang="scss">
    .objects-area {
        padding: 20px 45px;
        height: calc(100% - 40px);
    }
</style>
