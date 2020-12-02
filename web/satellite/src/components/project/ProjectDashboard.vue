// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">

        <h1 class="dashboard-area__title">{{projectName}} Dashboard</h1>
        <div
            class="dashboard-area__message-wrapper"
            @mouseenter="toggleVisibility"
            @mouseleave="toggleVisibility"
        >
            <InfoIcon class="dashboard-area__info-icon"/>
            <p class="dashboard-area__message" v-if="isVisible" >Expect an hour delay between network activity and the latest dashboard stats.</p>
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

import InfoIcon from '@/../static/images/common/infoTooltip.svg';

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
    },
})
export default class ProjectDashboard extends Vue {
    private isVisible: boolean = false;

    /**
     * Lifecycle hook after initial render.
     * Segment tracking is processed.
     */
    public mounted(): void {
        if (!this.$store.getters.selectedProject.id) {
            this.$router.push(RouteConfig.OnboardingTour.path);

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

    /**
     * Closes lag message.
     */
    public toggleVisibility(): void {
        this.isVisible = !this.isVisible;
    }
}
</script>

<style scoped lang="scss">
    .dashboard-area {
        padding: 50px 30px 30px 30px;
        font-family: 'font_regular', sans-serif;
        position: relative;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 22px;
            line-height: 27px;
            color: #384c65;
            margin: 10px 0 30px 0;
            display: inline-block;
        }

        &__message-wrapper {
            display: inline-block;
            position: absolute;
            border-radius: 5px;
            margin-left: 10px;
            background-size: 100% 100%;
        }

        &__message {
            width: 300px;
            color: #586c86;
            position: relative;
            left: 20px;
            bottom: 3px;
            font-size: 12px;
            line-height: 16px;
            background-image: url('../../../static/images/tooltipMessageBg.png');
            margin: 0;
            cursor: default;
            padding: 10px 10px 10px 30px;
            font-family: 'font_bold', sans-serif;
        }

        &__info-icon {
            position: absolute;
            top: 17px;
            left: 0px;
        }
    }
</style>
