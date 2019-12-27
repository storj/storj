// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-area">
        <NoBucketArea v-if="isNoBucketAreaShown"/>
        <div class="buckets-overflow" v-else>
            <div class="buckets-header">
                <p class="buckets-header__title">Buckets</p>
                <VHeader
                    class="buckets-header-component"
                    placeholder="Buckets"
                    :search="fetch"
                />
            </div>
            <div class="buckets-notification-container">
                <div class="buckets-notification">
                    <NotificationIcon/>
                    <p class="buckets-notification__text">Usage will appear within an hour of activity.</p>
                </div>
            </div>
            <div v-if="buckets.length" class="buckets-container">
                <SortingHeader/>
                <VList
                    :data-set="buckets"
                    :item-component="itemComponent"
                    :on-item-click="doNothing"
                />
                <VPagination
                    v-if="isPaginationShown"
                    :total-page-count="totalPageCount"
                    :on-page-click-callback="onPageClick"
                />
            </div>
            <div class="empty-search-result-area" v-if="isEmptySearchResultShown">
                <h1 class="empty-search-result-area__title">No results found</h1>
                <EmptySearchIcon class="empty-search-result-area__image"/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketItem from '@/components/buckets/BucketItem.vue';
import NoBucketArea from '@/components/buckets/NoBucketsArea.vue';
import SortingHeader from '@/components/buckets/SortingHeader.vue';
import VHeader from '@/components/common/VHeader.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

import EmptySearchIcon from '@/../static/images/buckets/emptySearch.svg';
import NotificationIcon from '@/../static/images/buckets/notification.svg';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { Bucket } from '@/types/buckets';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

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
        NotificationIcon,
        EmptySearchIcon,
    },
})
export default class BucketArea extends Vue {
    public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;

    public async mounted(): Promise<void> {
        await this.$store.dispatch(FETCH, 1);
    }

    public async beforeDestroy(): Promise<void> {
        await this.$store.dispatch(SET_SEARCH, '');
    }

    public doNothing(): void {
        // this method is used to mock prop function of common List
    }

    public get totalPageCount(): number {
        return this.$store.state.bucketUsageModule.page.pageCount;
    }

    public get totalCount(): number {
        return this.$store.getters.page.totalCount;
    }

    public get itemComponent() {
        return BucketItem;
    }

    public get buckets(): Bucket[] {
        return this.$store.getters.page.buckets;
    }

    public get search(): string {
        return this.$store.getters.cursor.search;
    }

    public get isNoBucketAreaShown(): boolean {
        return !this.totalCount && !this.search;
    }

    public get isPaginationShown(): boolean {
        return this.totalPageCount > 1;
    }

    public get isEmptySearchResultShown(): boolean {
        return !!(!this.totalPageCount && this.search);
    }

    public async fetch(searchQuery: string): Promise<void> {
        await this.$store.dispatch(SET_SEARCH, searchQuery);

        try {
            await this.$store.dispatch(FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets: ${error.message}`);
        }
    }

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
        padding-bottom: 100px;
        position: relative;
    }

    .buckets-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 32px 65px 20px 65px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            margin-right: 50px;
            margin-block-start: 0;
            margin-block-end: 0;
            user-select: none;
        }
    }

    .header-container.buckets-header-component {
        height: 55px !important;
    }

    .buckets-container,
    .buckets-notification-container {
        padding: 0 60px 0 60px;
    }

    .buckets-notification {
        width: calc(100% - 64px);
        display: flex;
        justify-content: flex-start;
        padding: 16px 32px;
        align-items: center;
        border-radius: 12px;
        background-color: #d0e3fe;
        margin-bottom: 25px;

        &__text {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            margin-left: 26px;
        }
    }

    .empty-search-result-area {
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            margin-top: 104px;
        }

        &__image {
            margin-top: 40px;
        }
    }

    @media screen and (max-width: 1024px) {

        .buckets-header {
            padding: 40px 40px 20px 40px;
        }

        .buckets-container,
        .buckets-notification-container {
            padding: 0 40px 0 40px;
        }
    }
</style>
