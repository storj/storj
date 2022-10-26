// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-view">
        <div class="buckets-view__title-area">
            <h1 class="buckets-view__title-area__title" aria-roledescription="title">Buckets</h1>
            <div class="new-bucket-button" :class="{ disabled: isLoading }" @click="onNewBucketButtonClick">
                <WhitePlusIcon class="new-bucket-button__icon" />
                <p class="new-bucket-button__label">New Bucket</p>
            </div>
        </div>

        <div class="buckets-view__divider" />

        <VLoader
            v-if="isLoading"
            width="100px"
            height="100px"
            class="buckets-view__loader"
        />

        <div v-if="!(isLoading || (bucketsPage.buckets && bucketsPage.buckets.length))" class="buckets-view__no-buckets-area">
            <EmptyBucketIcon class="buckets-view__no-buckets-area__image" />
            <h4 class="buckets-view__no-buckets-area__title">There are no buckets in this project</h4>
            <p class="buckets-view__no-buckets-area__body">Create a new bucket to upload files</p>
            <div class="new-bucket-button" :class="{ disabled: isLoading }" @click="onNewBucketButtonClick">
                <WhitePlusIcon class="new-bucket-button__icon" />
                <p class="new-bucket-button__label">New Bucket</p>
            </div>
        </div>

        <v-table
            v-if="!isLoading && bucketsPage.buckets && bucketsPage.buckets.length"
            class="buckets-view__list"
            :limit="bucketsPage.limit"
            :total-page-count="bucketsPage.pageCount"
            :items="bucketsPage.buckets"
            items-label="buckets"
            :on-page-click-callback="fetchBuckets"
            :total-items-count="bucketsPage.totalCount"
        >
            <template #head>
                <th class="buckets-view__list__sorting-header__name align-left">Name</th>
                <th class="buckets-view__list__sorting-header__date align-left">Date Added</th>
                <th class="buckets-view__list__sorting-header__empty" />
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
        <EncryptionBanner v-if="!isServerSideEncryptionBannerHidden" :hide="hideBanner" />
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData } from '@/utils/localData';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { BucketPage } from '@/types/buckets';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { AnalyticsHttpApi } from '@/api/analytics';

import VLoader from '@/components/common/VLoader.vue';
import BucketItem from '@/components/objects/BucketItem.vue';
import VTable from '@/components/common/VTable.vue';
import EncryptionBanner from '@/components/objects/EncryptionBanner.vue';

import WhitePlusIcon from '@/../static/images/common/plusWhite.svg';
import EmptyBucketIcon from '@/../static/images/objects/emptyBucket.svg';

// @vue/component
@Component({
    components: {
        VTable,
        WhitePlusIcon,
        EmptyBucketIcon,
        BucketItem,
        VLoader,
        EncryptionBanner,
    },
})
export default class BucketsView extends Vue {
    private readonly FILE_BROWSER_AG_NAME: string = 'Web file browser API key';

    public isLoading = true;
    public activeDropdown = -1;
    public isServerSideEncryptionBannerHidden = true;

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Sets bucket view.
     */
    public async mounted(): Promise<void> {
        this.isServerSideEncryptionBannerHidden = LocalData.getServerSideEncryptionBannerHidden();
        await this.setBucketsView();
    }

    @Watch('selectedProjectID')
    public async handleProjectChange(): Promise<void> {
        this.isLoading = true;

        await this.$store.dispatch(OBJECTS_ACTIONS.CLEAR);
        await this.setBucketsView();
    }

    /**
     * Sets buckets view when needed.
     */
    public async setBucketsView(): Promise<void> {
        try {
            await this.fetchBuckets();

            const wasDemoBucketCreated = LocalData.getDemoBucketCreatedStatus();

            if (this.bucketsPage.buckets.length && !wasDemoBucketCreated) {
                LocalData.setDemoBucketCreatedStatus();

                return;
            }

            if (!this.bucketsPage.buckets.length && wasDemoBucketCreated) {
                await this.removeTemporaryAccessGrant();

                return;
            }

            if (!this.bucketsPage.buckets.length && !wasDemoBucketCreated) {
                this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketCreation).path);
                await this.$router.push(RouteConfig.Buckets.with(RouteConfig.BucketCreation).path);
            }
        } catch (error) {
            await this.$notify.error(`Failed to setup Buckets view. ${error.message}`);
        } finally {
            this.isLoading = false;
        }
    }

    /**
     * Fetches bucket using api.
     */
    public async fetchBuckets(page = 1): Promise<void> {
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, page);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`);
        }
    }

    public onNewBucketButtonClick(): void {
        this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketCreation).path);
        this.$router.push(RouteConfig.Buckets.with(RouteConfig.BucketCreation).path);
    }

    /**
     * Removes temporary created access grant.
     */
    public async removeTemporaryAccessGrant(): Promise<void> {
        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, this.FILE_BROWSER_AG_NAME);
            await this.$store.dispatch(OBJECTS_ACTIONS.CLEAR);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Opens utils dropdown.
     */
    public openDropdown(key: number): void {
        if (this.activeDropdown === key) {
            this.activeDropdown = -1;

            return;
        }

        this.activeDropdown = key;
    }

    /**
     * Hides server-side encryption banner.
     */
    public hideBanner(): void {
        this.isServerSideEncryptionBannerHidden = true;
        LocalData.setServerSideEncryptionBannerHidden(true);
    }

    /**
     * Holds on bucket click. Proceeds to file browser.
     */
    public openBucket(bucketName: string): void {
        this.$store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, bucketName);
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_OPEN_BUCKET_MODAL_SHOWN);
    }

    /**
     * Returns fetched buckets page from store.
     */
    public get bucketsPage(): BucketPage {
        return this.$store.state.bucketUsageModule.page;
    }

    /**
     * Returns selected project id from store.
     */
    private get selectedProjectID(): string {
        return this.$store.getters.selectedProject.id;
    }
}
</script>

<style scoped lang="scss">
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
            background-color: #fcfcfc;
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
            margin-top: 40px;
            width: 100%;
        }
    }

    .disabled {
        pointer-events: none;
        background-color: #dadde5;
        border-color: #dadde5;
    }
</style>
