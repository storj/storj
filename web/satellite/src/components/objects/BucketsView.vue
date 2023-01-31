// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-view">
        <div class="buckets-view__title-area">
            <h1 class="buckets-view__title-area__title" aria-roledescription="title">Buckets</h1>
            <VButton
                v-if="promptForPassphrase"
                label="Set Encryption Passphrase ->"
                width="234px"
                height="40px"
                font-size="14px"
                :on-press="onSetClick"
            />
            <div v-else class="buckets-view-button" :class="{ disabled: isLoading }" @click="onCreateBucketClick">
                <WhitePlusIcon class="buckets-view-button__icon" />
                <p class="buckets-view-button__label">New Bucket</p>
            </div>
        </div>

        <div class="buckets-view__divider" />

        <BucketsTable :is-loading="isLoading" />
        <EncryptionBanner v-if="!isServerSideEncryptionBannerHidden" :hide="hideBanner" />
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData } from '@/utils/localData';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { BucketPage } from '@/types/buckets';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import EncryptionBanner from '@/components/objects/EncryptionBanner.vue';
import BucketsTable from '@/components/objects/BucketsTable.vue';
import VButton from '@/components/common/VButton.vue';

import WhitePlusIcon from '@/../static/images/common/plusWhite.svg';

// @vue/component
@Component({
    components: {
        WhitePlusIcon,
        BucketsTable,
        EncryptionBanner,
        VButton,
    },
})
export default class BucketsView extends Vue {
    public isLoading = true;
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

            if (!this.bucketsPage.buckets.length && !wasDemoBucketCreated && !this.promptForPassphrase) {
                this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_BUCKET_MODAL_SHOWN);
            }
        } catch (error) {
            await this.$notify.error(`Failed to setup Buckets view. ${error.message}`, AnalyticsErrorEventSource.BUCKET_PAGE);
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
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.BUCKET_PAGE);
        }
    }

    /**
     * Toggles create project passphrase modal visibility.
     */
    public onSetClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_PASSPHRASE_MODAL_SHOWN);
    }

    /**
     * Toggles create bucket modal visibility.
     */
    public onCreateBucketClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_BUCKET_MODAL_SHOWN);
    }

    /**
     * Hides server-side encryption banner.
     */
    public hideBanner(): void {
        this.isServerSideEncryptionBannerHidden = true;
        LocalData.setServerSideEncryptionBannerHidden(true);
    }

    /**
     * Returns fetched buckets page from store.
     */
    public get bucketsPage(): BucketPage {
        return this.$store.state.bucketUsageModule.page;
    }

    /**
     * Indicates if user should be prompt for passphrase.
     */
    public get promptForPassphrase(): boolean {
        return this.$store.state.objectsModule.promptForPassphrase;
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
