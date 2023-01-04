// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-table">
        <VHeader
            class="buckets-table__search"
            placeholder="Buckets"
            :search="searchBuckets"
            style-type="access"
        />
        <VLoader
            v-if="isLoading || searchLoading"
            width="100px"
            height="100px"
            class="buckets-view__loader"
        />
        <div v-if="isEmptyStateShown" class="buckets-table__no-buckets-area">
            <EmptyBucketIcon class="buckets-table__no-buckets-area__image" />
            <h4 class="buckets-table__no-buckets-area__title">There are no buckets in this project</h4>
            <p class="buckets-table__no-buckets-area__body">Create a new bucket to upload files</p>
            <div class="new-bucket-button" :class="{ disabled: isLoading }" @click="onNewBucketButtonClick">
                <WhitePlusIcon class="new-bucket-button__icon" />
                <p class="new-bucket-button__label">New Bucket</p>
            </div>
        </div>

        <div v-if="isNoSearchResultsShown" class="buckets-table__empty-search">
            <h1 class="buckets-table__empty-search__title">No results found</h1>
        </div>

        <v-table
            v-if="isTableShown"
            class="buckets-table__list"
            :limit="bucketsPage.limit"
            :total-page-count="bucketsPage.pageCount"
            items-label="buckets"
            :on-page-click-callback="fetchBuckets"
            :total-items-count="bucketsPage.totalCount"
            :selectable="false"
        >
            <template #head>
                <th class="align-left">Name</th>
                <th class="align-left">Storage</th>
                <th class="align-left">Bandwidth</th>
                <th class="align-left">Objects</th>
                <th class="align-left">Segments</th>
                <th class="align-left">Date Added</th>
                <th />
            </template>
            <template #body>
                <BucketItem
                    v-for="(bucket, key) in bucketsPage.buckets"
                    :key="key"
                    :item-data="bucket"
                    :dropdown-key="key"
                    :open-dropdown="openDropdown"
                    :is-dropdown-open="activeDropdown === key"
                    :show-guide="key === 0"
                    :on-click="() => openBucket(bucket.name)"
                />
            </template>
        </v-table>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { BucketPage } from '@/types/buckets';
import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import VTable from '@/components/common/VTable.vue';
import BucketItem from '@/components/objects/BucketItem.vue';
import VLoader from '@/components/common/VLoader.vue';
import VHeader from '@/components/common/VHeader.vue';

import WhitePlusIcon from '@/../static/images/common/plusWhite.svg';
import EmptyBucketIcon from '@/../static/images/objects/emptyBucket.svg';

const props = withDefaults(defineProps<{
    isLoading?: boolean,
}>(), {
    isLoading: false,
});

const activeDropdown = ref<number>(-1);
const searchLoading = ref<boolean>(false);
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const store = useStore();
const notify = useNotify();
const router = useRouter();

/**
 * Returns fetched buckets page from store.
 */
const bucketsPage = computed((): BucketPage => {
    return store.state.bucketUsageModule.page;
});

/**
 * Returns buckets search query.
 */
const searchQuery = computed((): string => {
    return store.getters.cursor.search;
});

/**
 * Indicates if buckets empty state is shown.
 */
const isEmptyStateShown = computed((): boolean => {
    return !props.isLoading && !searchLoading.value && !bucketsPage.value.buckets.length && !searchQuery.value;
});

/**
 * Indicates if empty search result is shown.
 */
const isNoSearchResultsShown = computed((): boolean => {
    return !props.isLoading && !searchLoading.value && !bucketsPage.value.buckets.length && !!searchQuery.value;
});

/**
 * Indicates if buckets table is shown.
 */
const isTableShown = computed((): boolean => {
    return !props.isLoading && !searchLoading.value && !!bucketsPage.value.buckets.length;
});

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return store.state.objectsModule.promptForPassphrase;
});

/**
 * Indicates if new encryption passphrase flow is enabled.
 */
const isNewEncryptionPassphraseFlowEnabled = computed((): boolean => {
    return store.state.appStateModule.isNewEncryptionPassphraseFlowEnabled;
});

/**
 * Starts bucket creation flow.
 */
function onNewBucketButtonClick(): void {
    analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketCreation).path);
    router.push(RouteConfig.Buckets.with(RouteConfig.BucketCreation).path);
}

/**
 * Fetches bucket using api.
 */
async function fetchBuckets(page = 1): Promise<void> {
    try {
        await store.dispatch(BUCKET_ACTIONS.FETCH, page);
    } catch (error) {
        await notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.BUCKET_TABLE);
    }
}

/**
 * Handles bucket search functionality.
 */
async function searchBuckets(searchQuery: string): Promise<void> {
    await store.dispatch(BUCKET_ACTIONS.SET_SEARCH, searchQuery);
    await analytics.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS);

    searchLoading.value = true;

    try {
        await store.dispatch(BUCKET_ACTIONS.FETCH, 1);
    } catch (error) {
        await notify.error(`Unable to fetch buckets: ${error.message}`, AnalyticsErrorEventSource.BUCKET_TABLE);
    }

    searchLoading.value = false;
}

/**
 * Opens utils dropdown.
 */
function openDropdown(key: number): void {
    if (activeDropdown.value === key) {
        activeDropdown.value = -1;

        return;
    }

    activeDropdown.value = key;
}

/**
 * Holds on bucket click. Proceeds to file browser.
 */
function openBucket(bucketName: string): void {
    store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, bucketName);
    if (isNewEncryptionPassphraseFlowEnabled.value && !promptForPassphrase.value) {
        analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);

        return;
    }

    store.commit(APP_STATE_MUTATIONS.TOGGLE_OPEN_BUCKET_MODAL_SHOWN);
}

onBeforeUnmount(() => {
    store.dispatch(BUCKET_ACTIONS.SET_SEARCH, '');
});
</script>

<style scoped lang="scss">
    .buckets-table {
        width: 100%;

        &__search {
            margin-bottom: 20px;
            height: 56px;
        }

        &__loader {
            margin-top: 100px;
        }

        &__no-buckets-area {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 80px 0;
            width: 100%;
            box-shadow: 0 0 32px rgb(0 0 0 / 4%);
            background-color: #fff;
            border-radius: 20px;

            &__image {
                margin-bottom: 60px;
            }

            &__title {
                font-family: 'font_medium', sans-serif;
                font-weight: 800;
                font-size: 18px;
                line-height: 16px;
                margin-bottom: 17px;
            }

            &__body {
                font-family: 'font_regular', sans-serif;
                font-weight: 400;
                font-size: 16px;
                line-height: 24px;
                margin-bottom: 24px;
            }
        }

        &__list {
            width: 100%;
        }

        &__empty-search {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px 0;
            background-color: #fff;
            border-radius: 10px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
            }
        }
    }

    .new-bucket-button {
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

    @media screen and (max-width: 875px) {

        :deep(thead) {
            display: none;
        }
    }
</style>
