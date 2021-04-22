// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';

@Component
export default class OnboardingTourArea extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public mounted(): void {
        if (this.userHasProject) {
            this.$router.push(RouteConfig.ProjectDashboard.path).catch(() => {return; });
        }
    }

    /**
     * Indicates if user has at least one project.
     */
    private get userHasProject(): boolean {
        return this.$store.state.projectsModule.projects.length > 0;
    }
}
</script>

<style scoped lang="scss">
    .tour-area {
        width: 100%;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 110px 0 80px 0;
    }
</style>
