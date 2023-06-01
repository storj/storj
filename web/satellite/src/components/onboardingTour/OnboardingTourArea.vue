// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { RouteConfig } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const route = useRoute();

onMounted(() => {
    // go back to all projects dashboard if there's no project selected, except on the pricing plan selection step.
    if (configStore.state.config.allProjectsDashboard
      && !projectsStore.state.selectedProject.id
      && route.name !== RouteConfig.PricingPlanStep.name
    ) {
        router.push(RouteConfig.AllProjectsDashboard.path);
    }
});
</script>

<style scoped lang="scss">
.tour-area {
    padding: 30px 0;
    box-sizing: border-box;
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
}

@media screen and (width <= 760px) {

    .tour-area {
        width: 88% !important;
    }
}
</style>
