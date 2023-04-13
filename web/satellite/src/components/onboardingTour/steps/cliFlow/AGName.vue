// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        :is-loading="isLoading"
        title="Create an Access Grant"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="permissions">
            <p class="permissions__msg">Access Grants are keys that allow access to upload, delete, and view your project’s data.</p>
            <VInput
                label="Access Grant Name"
                placeholder="Enter a name here..."
                :error="errorMessage"
                aria-roledescription="name"
                @setData="onChangeName"
            />
        </template>
    </CLIFlowContainer>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { RouteConfig } from '@/router';
import { AccessGrant } from '@/types/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify, useRouter } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import VInput from '@/components/common/VInput.vue';

import Icon from '@/../static/images/onboardingTour/accessGrant.svg';

const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const name = ref<string>('');
const errorMessage = ref<string>('');
const isLoading = ref<boolean>(false);

/**
 * Returns back route from store.
 */
const backRoute = computed((): string => {
    return appStore.state.viewsState.onbAGStepBackRoute;
});

/**
 * Changes name data from input value.
 * @param value
 */
function onChangeName(value: string): void {
    name.value = value.trim();
    errorMessage.value = '';
}

/**
 * Holds on back button click logic.
 * Navigates to previous screen.
 */
async function onBackClick(): Promise<void> {
    analytics.pageVisit(RouteConfig.OverviewStep.path);
    backRoute.value ?
        await router.push(backRoute.value).catch(() => {return; }) :
        await router.push({ name: RouteConfig.OverviewStep.name });
}

/**
 * Holds on next button click logic.
 */
async function onNextClick(): Promise<void> {
    if (isLoading.value) return;

    if (!name.value) {
        errorMessage.value = 'Access Grant name can\'t be empty';
        analytics.errorEventTriggered(AnalyticsErrorEventSource.ONBOARDING_NAME_STEP);

        return;
    }

    isLoading.value = true;

    let createdAccessGrant: AccessGrant;
    try {
        createdAccessGrant = await agStore.createAccessGrant(name.value, projectsStore.state.selectedProject.id);

        await notify.success('New clean access grant was generated successfully.');
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.ONBOARDING_NAME_STEP);
        return;
    } finally {
        isLoading.value = false;
    }

    appStore.setOnboardingCleanAPIKey(createdAccessGrant.secret);
    name.value = '';

    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGPermissions)).path);
    await router.push({ name: RouteConfig.AGPermissions.name });
}
</script>

<style scoped lang="scss">
    .permissions {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #4e4b66;
            margin-bottom: 20px;
        }
    }
</style>
