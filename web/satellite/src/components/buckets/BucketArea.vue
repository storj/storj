// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <NoBucketArea v-if="!totalCountOfBuckets && !search" />
        <div class="buckets-overflow" v-else>
            <div class="buckets-header">
                <p>Buckets</p>
                <HeaderComponent class="buckets-header-component" placeholder="Buckets" :search="fetch"/>
            </div>
            <div v-if="buckets.length" class="buckets-container">
                <SortingHeader />
                <List :dataSet="buckets" :itemComponent="itemComponent"/>
                <Pagination :totalPageCount="totalPageCount" :onPageClickCallback="onPageClick" />
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
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import BucketItem from '@/components/buckets/BucketItem.vue';
    import SortingHeader from '@/components/buckets/SortingHeader.vue';
    import NoBucketArea from '@/components/buckets/NoBucketsArea.vue';
    import HeaderComponent from '@/components/common/HeaderComponent.vue';
    import Pagination from '@/components/common/Pagination.vue';
    import List from '@/components/common/List.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import { BUCKET_USAGE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            EmptyState,
            SortingHeader,
            BucketItem,
            NoBucketArea,
            HeaderComponent,
            Pagination,
            List
        }
    })
    export default class BucketArea extends Vue {
        public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;

        public mounted(): void {
            this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, 1);
        }

        public get totalPageCount(): number {
            return this.$store.state.bucketUsageModule.page.pageCount;
        }

        public get totalCountOfBuckets(): number {
            return this.$store.state.bucketUsageModule.totalCount;
        }

        public get itemComponent() {
            return BucketItem;
        }

        public get buckets(): BucketUsage[] {
            return this.$store.state.bucketUsageModule.page.bucketUsages;
        }

        public get search(): string {
            return this.$store.state.bucketUsageModule.cursor.search;
        }

        public async fetch(searchQuery: string): Promise<void> {
            await this.$store.dispatch(BUCKET_USAGE_ACTIONS.SET_SEARCH, searchQuery);
            const bucketsResponse = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, 1);

            if (!bucketsResponse.isSuccess) {
                await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + bucketsResponse.errorMessage);
            }
        }
        public async onPageClick(page: number): Promise<void> {
            const response = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, page);
            if (!response.isSuccess) {
                await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + response.errorMessage);
            }
        }

    }
</script>

<style scoped lang="scss">
    .buckets-header {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        padding: 40px 40px 20px 60px;
        
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
    
    .buckets-container {
        padding: 0 40px 0 60px;
    }
    
    @media screen and (max-height: 880px) {
        .buckets-overflow {
            overflow-y: scroll;
            height: 600px;
        }
        
        .empty-container {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
        }
    }
    
    @media screen and (max-height: 700px) {
        .buckets-overflow {
            height: 570px;
        }
    }
</style>
