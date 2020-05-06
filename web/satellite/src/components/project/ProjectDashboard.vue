// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <h1 class="dashboard-area__title">Dashboard</h1>
        <div class="dashboard-area__top-area">
            <ProjectDetails/>
            <ProjectUsage/>
        </div>
        <BucketArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketArea from '@/components/project/buckets/BucketArea.vue';
import ProjectDetails from '@/components/project/ProjectDetails.vue';
import ProjectUsage from '@/components/project/usage/ProjectUsage.vue';

import { RouteConfig } from '@/router';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        BucketArea,
        ProjectDetails,
        ProjectUsage,
    },
})
export default class ProjectDashboard extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Segment tracking is processed.
     */
    public mounted(): void {
        if (!this.$store.getters.selectedProject.id) {
            this.$router.push(RouteConfig.OnboardingTour.path);

            return;
        }

        this.$segment.track(SegmentEvent.PROJECT_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
        });
    }
}
</script>

<style scoped lang="scss">
    .dashboard-area {
        padding: 40px 65px 80px 65px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            margin: 0 0 35px 0;
        }

        &__top-area {
            display: flex;
            align-items: flex-start;
            justify-content: space-between;
        }
    }
</style>
