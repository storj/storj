// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const router = useRouter();

onMounted(() => {
    if (configStore.state.config.allProjectsDashboard && !projectsStore.state.selectedProject.id) {
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
