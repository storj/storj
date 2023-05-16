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

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/router';
import { User } from '@/types/users';
import { AccessType } from '@/types/createAccessGrant';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import NewProjectIcon from '@/../static/images/navigation/newProject.svg';
import CreateAGIcon from '@/../static/images/navigation/createAccessGrant.svg';
import S3Icon from '@/../static/images/navigation/s3.svg';
import UploadInCLIIcon from '@/../static/images/navigation/uploadInCLI.svg';
import UploadInWebIcon from '@/../static/images/navigation/uploadInWeb.svg';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const route = useRoute();

const props = withDefaults(defineProps<{
    closeDropdowns?: () => void;
}>(), {
    closeDropdowns: () => {},
});

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Redirects to create project screen.
 */
function navigateToCreateAG(): void {
    analytics.eventTriggered(AnalyticsEvent.CREATE_AN_ACCESS_GRANT_CLICKED);
    props.closeDropdowns();

    analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
    router.push({
        name: RouteConfig.CreateAccessModal.name,
        query: { accessType: AccessType.AccessGrant },
    }).catch(() => {return;});
}

/**
 * Redirects to Create Access Modal with "s3" access type preselected
 */
function navigateToAccessGrantS3(): void {
    analytics.eventTriggered(AnalyticsEvent.CREATE_S3_CREDENTIALS_CLICKED);
    props.closeDropdowns();

    analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
    router.push({
        name: RouteConfig.CreateAccessModal.name,
        query: { accessType: AccessType.S3 },
    }).catch(() => {return;});
}

/**
 * Redirects to objects screen.
 */
function navigateToBuckets(): void {
    analytics.eventTriggered(AnalyticsEvent.UPLOAD_IN_WEB_CLICKED);
    props.closeDropdowns();
    analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);
    router.push(RouteConfig.Buckets.path).catch(() => {return;});
}

/**
 * Redirects to onboarding CLI flow screen.
 */
function navigateToCLIFlow(): void {
    analytics.eventTriggered(AnalyticsEvent.UPLOAD_USING_CLI_CLICKED);
    props.closeDropdowns();
    appStore.setOnboardingBackRoute(route.path);
    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
    router.push({ name: RouteConfig.AGName.name });
}

/**
 * Redirects to create access grant screen.
 */
function navigateToNewProject(): void {
    if (route.name !== RouteConfig.CreateProject.name) {
        analytics.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);

        const user: User = usersStore.state.user;
        const ownProjectsCount: number = projectsStore.projectsCount(user.id);

        if (!user.paidTier && user.projectLimit === ownProjectsCount) {
            appStore.updateActiveModal(MODALS.createProjectPrompt);
        } else {
            analytics.pageVisit(RouteConfig.CreateProject.path);
            appStore.updateActiveModal(MODALS.createProject);
        }
    }

    props.closeDropdowns();
}
</script>

<style scoped lang="scss">
    .dropdown-item:focus {
        background-color: #f5f6fa;
    }
</style>
