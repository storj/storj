// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <NoBucketArea v-if="!totalCount && !search" />
        <div class="buckets-overflow" v-else>
            <div class="buckets-header">
                <p>Buckets</p>
                <HeaderComponent class="buckets-header-component" placeHolder="Buckets" :search="fetch"/>
            </div>
            <div class="buckets-notification-container">
                <div class="buckets-notification">
                    <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <rect width="40" height="40" rx="10" fill="#2683FF"/>
                        <path d="M18.1489 17.043H21.9149V28H18.1489V17.043ZM20 12C20.5816 12 21.0567 12.1823 21.4255 12.5468C21.8085 12.8979 22 13.357 22 13.9241C22 14.4776 21.8085 14.9367 21.4255 15.3013C21.0567 15.6658 20.5816 15.8481 20 15.8481C19.4184 15.8481 18.9362 15.6658 18.5532 15.3013C18.1844 14.9367 18 14.4776 18 13.9241C18 13.357 18.1844 12.8979 18.5532 12.5468C18.9362 12.1823 19.4184 12 20 12Z" fill="#F5F6FA"/>
                    </svg>
                    <p class="buckets-notification__text">Usage will appear within an hour of activity.</p>
                </div>
            </div>
            <div v-if="buckets.length" class="buckets-container">
                <SortingHeader />
                <List :dataSet="buckets" :itemComponent="itemComponent" :onItemClick="doNothing"/>
                <Pagination v-if="totalPageCount > 1" :totalPageCount="totalPageCount" :onPageClickCallback="onPageClick" />
            </div>
            <EmptyState
                class="empty-container"
                v-if="!totalPageCount && search"
                mainTitle="Nothing found :("
                :imageSource="emptyImage" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketItem from '@/components/buckets/BucketItem.vue';
import NoBucketArea from '@/components/buckets/NoBucketsArea.vue';
import SortingHeader from '@/components/buckets/SortingHeader.vue';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import HeaderComponent from '@/components/common/HeaderComponent.vue';
import List from '@/components/common/List.vue';
import Pagination from '@/components/common/Pagination.vue';

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
        HeaderComponent,
        Pagination,
        List,
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
        
        p {
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
