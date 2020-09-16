// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <div class="dashboard-area__title-area">
            <h1 class="dashboard-area__title-area__title">Project Dashboard</h1>
            <a
                class="dashboard-area__title-area__link"
                :href="projectLimitsIncreaseRequestURL"
                target="_blank"
                rel="noopener noreferrer"
            >
                Request Limit Increase ->
            </a>
        </div>
        <ProjectUsage/>
        <BucketArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketArea from '@/components/project/buckets/BucketArea.vue';
import ProjectUsage from '@/components/project/usage/ProjectUsage.vue';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        BucketArea,
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

        const defaultProjectLimit: number = parseInt(MetaUtils.getMetaContent('default-project-limit'));
        if (defaultProjectLimit && this.$store.getters.userProjectsCount < defaultProjectLimit) {
            this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
        }

        this.$segment.track(SegmentEvent.PROJECT_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
        });
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

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 20px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #384b65;
                margin: 0;
            }

            &__link {
                font-size: 14px;
                line-height: 14px;
                color: #2683ff;
            }
        }
    }
</style>
