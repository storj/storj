// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-area">
        <NoBucketArea v-if="isNoBucketAreaShown" />
        <div v-else class="buckets-overflow">
            <div class="buckets-header">
                <p class="buckets-header__title">Usage per bucket</p>
                <VHeader
                    class="buckets-header-component"
                    placeholder="Buckets"
                    :search="fetch"
                />
            </div>
            <div v-if="buckets.length" class="buckets-container">
                <SortingHeader />
                <VList
                    class="buckets-list"
                    :data-set="buckets"
                    :item-component="itemComponent"
                    :on-item-click="doNothing"
                />
            </div>
            <div v-if="isEmptySearchResultShown" class="empty-search-result-area">
                <h1 class="empty-search-result-area__title">No results found</h1>
            </div>
        </div>
        <div v-if="isPaginationShown" class="buckets-area__pagination-area">
            <VPagination
                :total-page-count="totalPageCount"
                :on-page-click-callback="onPageClick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VHeader from '@/components/common/VHeader.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';
import BucketItem from '@/components/project/buckets/BucketItem.vue';
import NoBucketArea from '@/components/project/buckets/NoBucketsArea.vue';
import SortingHeader from '@/components/project/buckets/SortingHeader.vue';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { Bucket } from '@/types/buckets';

const {FETCH, SET_SEARCH} = BUCKET_ACTIONS;

// @vue/component
@Component({
    components: {
        SortingHeader,
        NoBucketArea,
        VHeader,
        VPagination,
        VList,
    },
})
export default class BucketArea extends Vue {
    /**
     * Lifecycle hook before component destruction where buckets search query is cleared.
     */
    public async beforeDestroy(): Promise<void> {
        await this.$store.dispatch(SET_SEARCH, '');
    }

    /**
     * Mock function for buckets list.
     */
    public doNothing(): void {
        // this method is used to mock prop function of common List
    }

    /**
     * Returns buckets total page count.
     */
    public get totalPageCount(): number {
        return this.$store.state.bucketUsageModule.page.pageCount;
    }

    /**
     * Returns buckets total count.
     */
    public get totalCount(): number {
        return this.$store.getters.page.totalCount;
    }

    /**
     * Returns BucketItem for common list.
     */
    public get itemComponent(): typeof BucketItem {
        return BucketItem;
    }

    /**
     * Returns buckets list of current page.
     */
    public get buckets(): Bucket[] {
        return this.$store.getters.page.buckets;
    }

    /**
     * Returns buckets search query.
     */
    public get search(): string {
        return this.$store.getters.cursor.search;
    }

    /**
     * Indicates if no bucket area is shown.
     */
    public get isNoBucketAreaShown(): boolean {
        return !this.totalCount && !this.search;
    }

    /**
     * Indicates if pagination is shown.
     */
    public get isPaginationShown(): boolean {
        return this.totalPageCount > 1;
    }

    /**
     * Indicates if empty bucket search is shown.
     */
    public get isEmptySearchResultShown(): boolean {
        return !!(!this.totalPageCount && this.search);
    }

    /**
     * Fetches buckets depending on search query.
     */
    public async fetch(searchQuery: string): Promise<void> {
        await this.$store.dispatch(SET_SEARCH, searchQuery);

        try {
            await this.$store.dispatch(FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets: ${error.message}`);
        }
    }

    /**
     * Fetches buckets depends on page index.
     */
    public async onPageClick(page: number): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, page);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets: ${error.message}`);
        }
    }
}
</script>

<style scoped lang="scss">
    .buckets-area {
        margin-top: 30px;
        position: relative;

        &__pagination-area {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: flex-start;
        }
    }

    .buckets-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 10px;

        &__title {
            margin: 0;
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 16px;
            color: #1b2533;
            white-space: nowrap;
        }
    }

    .header-container.buckets-header-component {
        height: 55px !important;
    }

    .buckets-container {
        background-color: #fff;
        border-radius: 6px;
        padding-bottom: 20px;
    }

    .empty-search-result-area {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 20px 0;
        background-color: #fff;
        border-radius: 6px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
        }
    }

    .buckets-list {
        padding-top: 20px;
    }

    ::v-deep .pagination-container {
        padding-left: 0;
    }
</style>
