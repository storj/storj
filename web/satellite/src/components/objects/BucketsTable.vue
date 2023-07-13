// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-table">
        <VSearch class="buckets-table__search" :search="searchBuckets" />
        <VLoader
            v-if="isLoading || searchLoading"
            width="100px"
            height="100px"
            class="buckets-view__loader"
        />
        <div v-if="isEmptyStateShown" class="buckets-table__no-buckets-area">
            <EmptyBucketIcon class="buckets-table__no-buckets-area__image" />
            <CreateBucketIcon class="buckets-table__no-buckets-area__small-image" />
            <h4 class="buckets-table__no-buckets-area__title">There are no buckets in this project</h4>
            <p class="buckets-table__no-buckets-area__body">Create a new bucket to upload files</p>
            <div class="new-bucket-button" :class="{ disabled: isLoading }" @click="onCreateBucketClick">
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
            :on-page-change="fetchBuckets"
            :total-items-count="bucketsPage.totalCount"
            :selectable="false"
        >
            <template #head>
                <th class="align-left">Name</th>
                <th class="align-left">Storage</th>
                <th class="align-left">Egress</th>
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
        <VOverallLoader v-if="overallLoading" />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';
import { useRouter } from 'vue-router';

import { BucketPage } from '@/types/buckets';
import { RouteConfig } from '@/types/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { EdgeCredentials } from '@/types/accessGrants';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VTable from '@/components/common/VTable.vue';
import BucketItem from '@/components/objects/BucketItem.vue';
import VLoader from '@/components/common/VLoader.vue';
import VOverallLoader from '@/components/common/VOverallLoader.vue';
import VSearch from '@/components/common/VSearch.vue';

import WhitePlusIcon from '@/../static/images/common/plusWhite.svg';
import EmptyBucketIcon from '@/../static/images/objects/emptyBucket.svg';
import CreateBucketIcon from '@/../static/images/buckets/createBucket.svg';

const props = withDefaults(defineProps<{
    isLoading?: boolean,
}>(), {
    isLoading: false,
});

const activeDropdown = ref<number>(-1);
const overallLoading = ref<boolean>(false);
const searchLoading = ref<boolean>(false);
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

/**
 * Returns fetched buckets page from store.
 */
const bucketsPage = computed((): BucketPage => {
    return bucketsStore.state.page;
});

/**
 * Returns buckets search query.
 */
const searchQuery = computed((): string => {
    return bucketsStore.state.cursor.search;
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
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Toggles create bucket modal visibility.
 */
function onCreateBucketClick(): void {
    appStore.updateActiveModal(MODALS.createBucket);
}

/**
 * Fetches bucket using api.
 */
async function fetchBuckets(page = 1, limit: number): Promise<void> {
    try {
        await bucketsStore.getBuckets(page, projectsStore.state.selectedProject.id, limit);
    } catch (error) {
        notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.BUCKET_TABLE);
    }
}

/**
 * Handles bucket search functionality.
 */
async function searchBuckets(searchQuery: string): Promise<void> {
    bucketsStore.setBucketsSearch(searchQuery);
    analytics.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS);

    searchLoading.value = true;

    try {
        await bucketsStore.getBuckets(1, projectsStore.state.selectedProject.id);
    } catch (error) {
        notify.error(`Unable to fetch buckets: ${error.message}`, AnalyticsErrorEventSource.BUCKET_TABLE);
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
async function openBucket(bucketName: string): Promise<void> {
    bucketsStore.setFileComponentBucketName(bucketName);
    if (!promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            overallLoading.value = true;

            try {
                await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
                overallLoading.value = false;
            } catch (error) {
                notify.error(error.message, AnalyticsErrorEventSource.BUCKET_TABLE);
                overallLoading.value = false;
                return;
            }
        }

        analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);

        return;
    }

    appStore.updateActiveModal(MODALS.enterBucketPassphrase);
}

onBeforeUnmount(() => {
    bucketsStore.setBucketsSearch('');
});
</script>

<style scoped lang="scss">
    .buckets-table {
        width: 100%;

        &__search {
            margin-bottom: 20px;
        }

        &__loader {
            margin-top: 100px;
        }

        &__no-buckets-area {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 80px 20px;
            width: calc(100% - 40px);
            box-shadow: 0 0 32px rgb(0 0 0 / 4%);
            background-color: #fff;
            border-radius: 20px;

            &__image {
                margin-bottom: 60px;

                @media screen and (width <= 600px) {
                    display: none;
                }
            }

            &__small-image {
                display: none;
                margin-bottom: 60px;

                @media screen and (width <= 600px) {
                    display: block;
                }
            }

            &__title {
                font-family: 'font_medium', sans-serif;
                font-weight: 800;
                font-size: 18px;
                line-height: 16px;
                margin-bottom: 17px;
                text-align: center;
            }

            &__body {
                font-family: 'font_regular', sans-serif;
                font-weight: 400;
                font-size: 16px;
                line-height: 24px;
                margin-bottom: 24px;
                text-align: center;
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

    @media screen and (width <= 875px) {

        :deep(thead) {
            display: none;
        }
    }
</style>
