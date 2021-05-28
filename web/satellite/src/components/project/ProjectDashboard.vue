// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <div class="dashboard-area__header-wrapper">
            <h1 class="dashboard-area__header-wrapper__title">{{projectName}} Dashboard</h1>
            <p class="dashboard-area__header-wrapper__message">
                Expect a delay of a few hours between network activity and the latest dashboard stats.
            </p>
        </div>
        <ProjectUsage/>
        <ProjectSummary/>
        <BucketArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketArea from '@/components/project/buckets/BucketArea.vue';
import ProjectSummary from '@/components/project/summary/ProjectSummary.vue';
import ProjectUsage from '@/components/project/usage/ProjectUsage.vue';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        BucketArea,
        ProjectUsage,
        ProjectSummary,
    },
})
export default class ProjectDashboard extends Vue {
    /**
     * Lifecycle hook after initial render.
     */
    public mounted(): void {
        if (!this.$store.getters.selectedProject.id) {
            this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

            return;
        }

        const projectLimit: number = this.$store.getters.user.projectLimit;
        if (projectLimit && this.$store.getters.projectsCount < projectLimit) {
            this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
        }
    }

    /**
     * Returns selected project name.
     */
    public get projectName(): string {
        return this.$store.getters.selectedProject.name;
    }

    /**
     * Returns project limits increase request url from config.
     */
    public get projectLimitsIncreaseRequestURL(): string {
        return MetaUtils.getMetaContent('project-limits-increase-request-url');
    }
}
</script>

<style scoped lang="scss">
    .dashboard-area {
        padding: 50px 30px 60px 30px;
        font-family: 'font_regular', sans-serif;

        &__header-wrapper {
            display: flex;
            flex-direction: column;
            margin: 10px 0 30px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #384b65;
                margin: 0;
            }

            &__message {
                font-size: 16px;
                line-height: 20px;
                color: #384b65;
                margin: 10px 0 0 0;
            }
        }
    }
</style>
