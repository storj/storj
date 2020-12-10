// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <div class="dashboard-area__header-wrapper">
            <h1 class="dashboard-area__title">{{projectName}} Dashboard</h1>
            <VInfo
                class="dashboard-area__tooltip__wrapper"
                bold-text="Expect a delay of a few hours between network activity and the latest dashboard stats.">
                <InfoIcon class="dashboard-area__tooltip__icon"/>
            </VInfo>
        </div>
        <ProjectUsage/>
        <ProjectSummary/>
        <BucketArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/components/common/VInfo.vue';
import BucketArea from '@/components/project/buckets/BucketArea.vue';
import ProjectSummary from '@/components/project/summary/ProjectSummary.vue';
import ProjectUsage from '@/components/project/usage/ProjectUsage.vue';

import InfoIcon from '@/../static/images/common/infoTooltipSm.svg';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        BucketArea,
        InfoIcon,
        ProjectUsage,
        ProjectSummary,
        VInfo,
    },
})
export default class ProjectDashboard extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Segment tracking is processed.
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

        this.$segment.track(SegmentEvent.PROJECT_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
        });
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
        padding: 50px 30px 30px 30px;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 22px;
            line-height: 27px;
            color: #384b65;
            margin: 0 0 30px 0;
        }

        &__header-wrapper {
            display: flex;
            margin-top: 10px;
        }

        &__tooltip {

            &__wrapper {
                margin: 7px 0 0 10px;

                /deep/ .info__message-box {
                    background-image: url('../../../static/images/tooltipMessageBg.png');
                    min-width: 300px;
                    text-align: left;
                    left: 195px;
                    bottom: 15px;
                    padding: 10px 10px 10px 35px;

                    &__text {

                        &__bold-text {
                            font-family: 'font_medium', sans-serif;
                            color: #354049;
                        }
                    }
                }
            }
        }
    }
</style>
