// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-area">
        <NoBucketArea v-if="isNoBucketAreaShown"/>
        <div class="buckets-overflow" v-else>
            <div class="buckets-header">
                <p class="buckets-header__title">Buckets Usage</p>
                <VHeader
                    class="buckets-header-component"
                    placeholder="Buckets"
                    :search="fetch"
                />
            </div>
            <div v-if="buckets.length" class="buckets-container">
                <SortingHeader/>
                <VList
                    :data-set="buckets"
                    :item-component="itemComponent"
                    :on-item-click="doNothing"
                />
            </div>
            <div class="empty-search-result-area" v-if="isEmptySearchResultShown">
                <h1 class="empty-search-result-area__title">No results found</h1>
            </div>
        </div>
        <div class="buckets-area__pagination-area" v-if="isPaginationShown">
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

const {
    FETCH,
    SET_SEARCH,
    CLEAR,
} = BUCKET_ACTIONS;

@Component({
    components: {
        SortingHeader,
        BucketItem,
        NoBucketArea,
        VHeader,
        VPagination,
        VList,
    },
})
export default class BucketArea extends Vue {
    /**
     * Lifecycle hook after initial render where buckets list is fetched.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) {
            return;
        }

        try {
            await this.$store.dispatch(FETCH, 1);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

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
    public get itemComponent() {
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
        padding: 25px 30px 15px 30px;
        background-color: #fff;
        border-top-left-radius: 6px;
        border-top-right-radius: 6px;

        &__title {
            white-space: nowrap;
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 18px;
            color: #354049;
            margin: 0 50px 0 0;
        }
    }

    .header-container.buckets-header-component {
        height: 55px !important;
    }

    .buckets-container {
        padding: 0 30px;
        background-color: #fff;
        border-bottom-left-radius: 6px;
        border-bottom-right-radius: 6px;
    }

    .empty-search-result-area {
        display: flex;
        align-items: center;
        justify-content: center;
        padding-bottom: 20px;
        background-color: #fff;
        border-bottom-left-radius: 6px;
        border-bottom-right-radius: 6px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
        }
    }

    /deep/ .pagination-container {
        padding-left: 0;
    }
</style>
