// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
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
            <EmptyState
                class="empty-container"
                v-if="isEmptySearchResultShown"
                main-title="Nothing found :("
                :image-source="emptyImage"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketItem from '@/components/buckets/BucketItem.vue';
import NoBucketArea from '@/components/buckets/NoBucketsArea.vue';
import SortingHeader from '@/components/buckets/SortingHeader.vue';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import VHeader from '@/components/common/VHeader.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

import NotificationIcon from '@/../static/images/buckets/notification.svg';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { Bucket } from '@/types/buckets';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

const {
    FETCH,
    SET_SEARCH,
} = BUCKET_ACTIONS;

@Component({
    components: {
        EmptyState,
        SortingHeader,
        BucketItem,
        NoBucketArea,
        VHeader,
        VPagination,
        VList,
        NotificationIcon,
    },
})
export default class BucketArea extends Vue {
    public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;

    public mounted(): void {
        this.$store.dispatch(FETCH, 1);
    }

    public doNothing(): void {
        // this method is used to mock prop function of common List
    }

    public get totalPageCount(): number {
        return this.$store.getters.page.pageCount;
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
            await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch buckets: ${error.message}`);
        }
    }

    public async onPageClick(page: number): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, page);
        } catch (error) {
            await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch buckets: ${error.message}`);
        }
    }
}
</script>

<style scoped lang="scss">
    .buckets-header {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        padding: 40px 60px 20px 60px;
        
        &__title {
            font-family: 'font_bold';
            font-size: 32px;
            line-height: 39px;
            color: #384B65;
            margin-right: 50px;
            margin-block-start: 0;
            margin-block-end: 0;
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
        background-color: #D0E3FE;
        margin-bottom: 25px;

        &__text {
            font-family: 'font_medium';
            font-size: 14px;
            margin-left: 26px;
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

    @media screen and (max-height: 880px) {
        .buckets-overflow {
            overflow-y: scroll;
            height: 750px;
        }
        
        .empty-container {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
        }
    }

    @media screen and (max-height: 853px) {
        .buckets-overflow {
            height: 700px;
        }
    }

    @media screen and (max-height: 805px) {
        .buckets-overflow {
            height: 630px;
        }
    }

    @media screen and (max-height: 740px) {
        .buckets-overflow {
            height: 600px;
        }
    }
    
    @media screen and (max-height: 700px) {
        .buckets-overflow {
            height: 570px;
        }
    }
</style>
