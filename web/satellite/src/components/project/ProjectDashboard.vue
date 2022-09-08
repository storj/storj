// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <div class="dashboard-area__header-wrapper">
            <h1 class="dashboard-area__header-wrapper__title" aria-roledescription="title">{{ projectName }} Dashboard</h1>
            <p class="dashboard-area__header-wrapper__message">
                Expect a delay of a few hours between network activity and the latest dashboard stats.
            </p>
        </div>
        <ProjectUsage />
        <ProjectSummary :is-data-fetching="isSummaryDataFetching" />
        <div v-if="areBucketsFetching" class="dashboard-area__container">
            <p class="dashboard-area__container__title">Buckets</p>
            <VLoader />
        </div>
        <BucketArea v-else />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { AnalyticsHttpApi } from '@/api/analytics';

import ProjectUsage from '@/components/project/usage/ProjectUsage.vue';
import ProjectSummary from '@/components/project/summary/ProjectSummary.vue';
import BucketArea from '@/components/project/buckets/BucketArea.vue';
import VLoader from '@/components/common/VLoader.vue';

// @vue/component
@Component({
    components: {
        BucketArea,
        ProjectUsage,
        ProjectSummary,
        VLoader,
    },
})
export default class ProjectDashboard extends Vue {
    public areBucketsFetching = true;
    public isSummaryDataFetching = true;

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Fetches buckets, usage rollup, project members and access grants.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) {
            this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

            return;
        }

        const FIRST_PAGE = 1;

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, FIRST_PAGE);

            this.areBucketsFetching = false;

            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, FIRST_PAGE);

            this.isSummaryDataFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Returns selected project name.
     */
    public get projectName(): string {
        return this.$store.getters.selectedProject.name;
    }
}
</script>

<style scoped lang="scss">
    .dashboard-area {
        padding: 30px 30px 60px;
        height: calc(100% - 90px);
        font-family: 'font_regular', sans-serif;

        &__header-wrapper {
            display: flex;
            flex-direction: column;
            margin: 10px 0 30px;

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
                margin: 10px 0 0;
            }
        }

        &__container {
            background-color: #fff;
            border-radius: 6px;
            padding: 20px;
            margin-top: 30px;

            &__title {
                margin: 0 0 20px;
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 16px;
                color: #1b2533;
            }
        }
    }
</style>
