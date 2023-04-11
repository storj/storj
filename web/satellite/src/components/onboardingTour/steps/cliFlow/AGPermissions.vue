// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        :is-loading="isLoading || areBucketNamesFetching"
        title="Access Permissions"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="permissions">
            <div class="permissions__select">
                <p class="permissions__select__label">Select buckets to grant permission:</p>
                <VLoader v-if="areBucketNamesFetching" width="50px" height="50px" />
                <BucketsSelection v-else />
            </div>
            <div class="permissions__bucket-bullets">
                <div
                    v-for="(name, index) in selectedBucketNames"
                    :key="index"
                    class="permissions__bucket-bullets__container"
                >
                    <BucketNameBullet :name="name" />
                </div>
            </div>
            <div class="permissions__select">
                <p class="permissions__select__label">Choose permissions to allow:</p>
                <PermissionsSelect />
            </div>
            <div class="permissions__select">
                <p class="permissions__select__label">Duration of this access grant:</p>
                <DurationSelection />
            </div>
        </template>
    </CLIFlowContainer>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';

import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import PermissionsSelect from '@/components/onboardingTour/steps/cliFlow/PermissionsSelect.vue';
import BucketNameBullet from '@/components/onboardingTour/steps/cliFlow/permissions/BucketNameBullet.vue';
import BucketsSelection from '@/components/onboardingTour/steps/cliFlow/permissions/BucketsSelection.vue';
import VLoader from '@/components/common/VLoader.vue';
import DurationSelection from '@/components/onboardingTour/steps/cliFlow/permissions/DurationSelection.vue';

import Icon from '@/../static/images/onboardingTour/accessGrant.svg';

const appStore = useAppStore();
const store = useStore();
const notify = useNotify();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const worker = ref<Worker| null>(null);
const areBucketNamesFetching = ref<boolean>(true);
const isLoading = ref<boolean>(true);

/**
 * Returns selected bucket names.
 */
const selectedBucketNames = computed((): string[] => {
    return store.state.accessGrantsModule.selectedBucketNames;
});

/**
 * Returns clean API key from store.
 */
const cleanAPIKey = computed((): string => {
    return appStore.state.viewsState.onbCleanApiKey;
});

/**
 * Returns download permission from store.
 */
const storedIsDownload = computed((): boolean => {
    return store.state.accessGrantsModule.isDownload;
});

/**
 * Returns upload permission from store.
 */
const storedIsUpload = computed((): boolean => {
    return store.state.accessGrantsModule.isUpload;
});

/**
 * Returns list permission from store.
 */
const storedIsList = computed((): boolean => {
    return store.state.accessGrantsModule.isList;
});

/**
 * Returns delete permission from store.
 */
const storedIsDelete = computed((): boolean => {
    return store.state.accessGrantsModule.isDelete;
});

/**
 * Returns not before date permission from store.
 */
const notBeforePermission = computed((): Date | null => {
    return store.state.accessGrantsModule.permissionNotBefore;
});

/**
 * Returns not after date permission from store.
 */
const notAfterPermission = computed((): Date | null => {
    return store.state.accessGrantsModule.permissionNotAfter;
});

/**
 * Sets local worker with worker instantiated in store.
 * Also sets worker's onmessage and onerror logic.
 */
function setWorker(): void {
    worker.value = store.state.accessGrantsModule.accessGrantsWebWorker;

    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.ONBOARDING_PERMISSIONS_STEP);
        };
    }
}

/**
 * Holds on back button click logic.
 * Navigates to previous screen.
 */
async function onBackClick(): Promise<void> {
    if (isLoading.value) return;

    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
    await router.push({ name: RouteConfig.AGName.name });
}

/**
 * Holds on next button click logic.
 */
async function onNextClick(): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;

    try {
        const restrictedKey = await generateRestrictedKey();
        appStore.setOnboardingAPIKey(restrictedKey);

        await notify.success('Restrictions were set successfully.');
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.ONBOARDING_PERMISSIONS_STEP);
        return;
    } finally {
        isLoading.value = false;
    }

    appStore.setOnboardingAPIKeyStepBackRoute(router.currentRoute.path);
    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.APIKey)).path);
    await router.push({ name: RouteConfig.APIKey.name });
}

/**
 * Generates and returns restricted key from clean API key.
 */
async function generateRestrictedKey(): Promise<string> {
    if (!worker.value) {
        throw new Error('Worker is not defined');
    }

    let permissionsMsg = {
        'type': 'SetPermission',
        'isDownload': storedIsDownload.value,
        'isUpload': storedIsUpload.value,
        'isList': storedIsList.value,
        'isDelete': storedIsDelete.value,
        'buckets': selectedBucketNames.value,
        'apiKey': cleanAPIKey.value,
    };

    if (notBeforePermission.value) permissionsMsg = Object.assign(
        permissionsMsg, { 'notBefore': notBeforePermission.value.toISOString() },
    );
    if (notAfterPermission.value) permissionsMsg = Object.assign(
        permissionsMsg, { 'notAfter': notAfterPermission.value.toISOString() },
    );

    await worker.value.postMessage(permissionsMsg);

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (grantEvent.data.error) {
        throw new Error(grantEvent.data.error);
    }

    analytics.eventTriggered(AnalyticsEvent.API_KEY_GENERATED);

    return grantEvent.data.value;
}

/**
 * Lifecycle hook after initial render.
 * Checks if clean api key was generated during previous step.
 * Fetches all existing bucket names.
 * Initializes web worker's onmessage functionality.
 */
onMounted(async (): Promise<void> => {
    if (!cleanAPIKey.value) {
        isLoading.value = false;
        await onBackClick();

        return;
    }

    setWorker();

    try {
        await store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);

        areBucketNamesFetching.value = false;
    } catch (error) {
        await notify.error(`Unable to fetch all bucket names. ${error.message}`, AnalyticsErrorEventSource.ONBOARDING_PERMISSIONS_STEP);
    }

    isLoading.value = false;
});
</script>

<style scoped lang="scss">
    .permissions {

        &__select {
            width: 287px;
            padding: 0 98.5px;

            &__label {
                font-family: 'font_medium', sans-serif;
                margin: 20px 0 8px;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
            }
        }

        &__bucket-bullets {
            display: flex;
            align-items: flex-start;
            width: calc(100% - 197px);
            padding: 0 98.5px;
            flex-wrap: wrap;

            &__container {
                display: flex;
                margin-top: 5px;
            }
        }
    }

    :deep(.buckets-selection),
    :deep(.duration-selection) {
        width: 287px;
        margin-left: 0;
    }
</style>
