// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div tabindex="0" class="dropdown-item" aria-roledescription="create-project-route" @click.stop="navigateToNewProject" @keyup.enter="navigateToNewProject">
            <NewProjectIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">New Project</h2>
                <p class="dropdown-item__text__label">Create a new project.</p>
            </div>
        </div>
        <div tabindex="0" class="dropdown-item" aria-roledescription="create-ag-route" @click.stop="navigateToCreateAG" @keyup.enter="navigateToCreateAG">
            <CreateAGIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Create an Access Grant</h2>
                <p class="dropdown-item__text__label">Start the wizard to create a new access grant.</p>
            </div>
        </div>
        <div tabindex="0" class="dropdown-item" aria-roledescription="create-s3-credentials-route" @click.stop="navigateToAccessGrantS3" @keyup.enter="navigateToAccessGrantS3">
            <S3Icon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Create S3 Gateway Credentials</h2>
                <p class="dropdown-item__text__label">Start the wizard to generate S3 credentials.</p>
            </div>
        </div>
        <div tabindex="0" class="dropdown-item" aria-roledescription="objects-route" @click.stop="navigateToBuckets" @keyup.enter="navigateToBuckets">
            <UploadInWebIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Upload in Web</h2>
                <p class="dropdown-item__text__label">Start uploading files in the web browser.</p>
            </div>
        </div>
        <div tabindex="0" class="dropdown-item" aria-roledescription="cli-flow-route" @click.stop="navigateToCLIFlow" @keyup.enter="navigateToCLIFlow">
            <UploadInCLIIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Upload using CLI</h2>
                <p class="dropdown-item__text__label">Start guide for using the Uplink CLI.</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/router';
import { User } from '@/types/users';
import { AccessType } from '@/types/createAccessGrant';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import NewProjectIcon from '@/../static/images/navigation/newProject.svg';
import CreateAGIcon from '@/../static/images/navigation/createAccessGrant.svg';
import S3Icon from '@/../static/images/navigation/s3.svg';
import UploadInCLIIcon from '@/../static/images/navigation/uploadInCLI.svg';
import UploadInWebIcon from '@/../static/images/navigation/uploadInWeb.svg';

// @vue/component
@Component({
    components: {
        NewProjectIcon,
        CreateAGIcon,
        S3Icon,
        UploadInCLIIcon,
        UploadInWebIcon,
    },
})
export default class QuickStartLinks extends Vue {
    @Prop({ default: () => () => { return; } })
    public closeDropdowns: () => void;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Redirects to create project screen.
     */
    public navigateToCreateAG(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_AN_ACCESS_GRANT_CLICKED);
        this.closeDropdowns();

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
        this.$router.push({
            name: RouteConfig.CreateAccessModal.name,
            params: { accessType: AccessType.AccessGrant },
        }).catch(() => {return;});
    }

    /**
     * Redirects to Create Access Modal with "s3" access type preselected
     */
    public navigateToAccessGrantS3(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_S3_CREDENTIALS_CLICKED);
        this.closeDropdowns();

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
        this.$router.push({
            name: RouteConfig.CreateAccessModal.name,
            params: { accessType: AccessType.S3 },
        }).catch(() => {return;});
    }

    /**
     * Redirects to objects screen.
     */
    public navigateToBuckets(): void {
        this.analytics.eventTriggered(AnalyticsEvent.UPLOAD_IN_WEB_CLICKED);
        this.closeDropdowns();
        this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);
        this.$router.push(RouteConfig.Buckets.path).catch(() => {return;});
    }

    /**
     * Redirects to onboarding CLI flow screen.
     */
    public navigateToCLIFlow(): void {
        this.analytics.eventTriggered(AnalyticsEvent.UPLOAD_USING_CLI_CLICKED);
        this.closeDropdowns();
        this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_AG_NAME_STEP_BACK_ROUTE, this.$route.path);
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
        this.$router.push({ name: RouteConfig.AGName.name });
    }

    /**
     * Redirects to create access grant screen.
     */
    public navigateToNewProject(): void {
        if (this.$route.name !== RouteConfig.CreateProject.name) {
            this.analytics.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);

            const user: User = this.$store.getters.user;
            const ownProjectsCount: number = this.$store.getters.projectsCount;

            if (!user.paidTier && user.projectLimit === ownProjectsCount) {
                this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.createProjectPrompt);
            } else {
                this.analytics.pageVisit(RouteConfig.CreateProject.path);
                this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.createProject);
            }
        }

        this.closeDropdowns();
    }
}
</script>

<style scoped lang="scss">
    .dropdown-item:focus {
        background-color: #f5f6fa;
    }
</style>
