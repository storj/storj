// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-view">
        <div class="buckets-view__title-area">
            <h1 class="buckets-view__title-area__title" aria-roledescription="title">Buckets</h1>
            <div class="buckets-view-button" :class="{ disabled: isLoading }" @click="onCreateBucketClick">
                <WhitePlusIcon class="buckets-view-button__icon" />
                <p class="buckets-view-button__label">New Bucket</p>
            </div>
        </div>

        <div class="buckets-view__divider" />

        <BucketsTable :is-loading="isLoading" />
        <EncryptionBanner v-if="!isServerSideEncryptionBannerHidden" :hide="hideBanner" />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { LocalData } from '@/utils/localData';
import { BucketPage } from '@/types/buckets';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { RouteConfig } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';

import EncryptionBanner from '@/components/objects/EncryptionBanner.vue';
import BucketsTable from '@/components/objects/BucketsTable.vue';

import WhitePlusIcon from '@/../static/images/common/plusWhite.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const notify = useNotify();
const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isLoading = ref<boolean>(true);
const isServerSideEncryptionBannerHidden = ref<boolean>(true);

/**
 * Returns fetched buckets page from store.
 */
const bucketsPage = computed((): BucketPage => {
    return bucketsStore.state.page;
});

/**
 * Indicates if user should be prompt for passphrase.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns selected project id from store.
 */
const selectedProjectID = computed((): string => {
    return projectsStore.state.selectedProject.id;
});

/**
 * Sets buckets view when needed.
 */
async function setBucketsView(): Promise<void> {
    try {
        await fetchBuckets();

        const wasDemoBucketCreated = LocalData.getDemoBucketCreatedStatus();

        if (bucketsPage.value.buckets.length && !wasDemoBucketCreated) {
            LocalData.setDemoBucketCreatedStatus();

            return;
        }

        if (!bucketsPage.value.buckets.length && !wasDemoBucketCreated && !promptForPassphrase.value) {
            appStore.updateActiveModal(MODALS.createBucket);
        }
    } catch (error) {
        notify.error(`Failed to setup Buckets view. ${error.message}`, AnalyticsErrorEventSource.BUCKET_PAGE);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Fetches bucket using api.
 */
async function fetchBuckets(page = 1): Promise<void> {
    try {
        await bucketsStore.getBuckets(page, selectedProjectID.value);
    } catch (error) {
        notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.BUCKET_PAGE);
    }
}

/**
 * Toggles create bucket modal visibility.
 */
function onCreateBucketClick(): void {
    appStore.updateActiveModal(MODALS.createBucket);
}

/**
 * Hides server-side encryption banner.
 */
function hideBanner(): void {
    isServerSideEncryptionBannerHidden.value = true;
    LocalData.setServerSideEncryptionBannerHidden(true);
}

/**
 * Lifecycle hook after initial render.
 * Sets bucket view.
 */
onMounted(async (): Promise<void> => {
    if (configStore.state.config.allProjectsDashboard && !projectsStore.state.selectedProject.id) {
        await router.push(RouteConfig.AllProjectsDashboard.path);
        return;
    }

    isServerSideEncryptionBannerHidden.value = LocalData.getServerSideEncryptionBannerHidden();
    await setBucketsView();
});

watch(selectedProjectID, async () => {
    isLoading.value = true;

    bucketsStore.clear();
    await setBucketsView();
});
</script>

<style scoped lang="scss">
    .buckets-view-button {
        padding: 0 15px;
        height: 40px;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--c-blue-3);
        border-radius: 8px;
        cursor: pointer;

        &__label {
            font-family: 'font-medium', sans-serif;
            font-weight: 700;
            font-size: 13px;
            line-height: 20px;
            color: #fff;
            margin: 0 0 0 5px;
        }

        &__icon {
            color: #fff;
        }

        &:hover {
            background-color: #0000c2;
        }
    }

    .buckets-view {
        display: flex;
        flex-direction: column;
        align-items: center;
        background-color: #f5f6fa;

        &__title-area {
            width: 100%;
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-weight: 600;
                font-size: 28px;
                line-height: 34px;
                color: #232b34;
                margin: 0;
                text-align: left;
            }
        }

        &__divider {
            width: 100%;
            height: 1px;
            background: #dadfe7;
            margin: 24px 0;
        }
    }

    .disabled {
        pointer-events: none;
        background-color: #dadde5;
        border-color: #dadde5;
    }
</style>
