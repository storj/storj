// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <NoBucketArea v-if="!totalCountOfBuckets && !search" />
        <div class="buckets-overflow" v-else>
            <div class="buckets-header">
                <p>Buckets</p>
                <SearchArea/>
            </div>
            <div v-if="buckets.length" class="buckets-container">
                <table>
                    <SortingHeader />
                    <BucketItem v-for="(bucket, index) in buckets" v-bind:bucket="bucket" v-bind:key="index" />
                </table>
                <PaginationArea />
            </div>
            <EmptyState
                class="empty-container"
                v-if="!pages && search"
                mainTitle="Nothing found :("
                :imageSource="emptyImage" />
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import SearchArea from '@/components/buckets/SearchArea.vue';
    import BucketItem from '@/components/buckets/BucketItem.vue';
    import PaginationArea from '@/components/buckets/PaginationArea.vue';
    import SortingHeader from '@/components/buckets/SortingHeader.vue';
    import NoBucketArea from '@/components/buckets/NoBucketsArea.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import { BUCKET_USAGE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            EmptyState,
            SearchArea,
            SortingHeader,
            BucketItem,
            PaginationArea,
            NoBucketArea,
        }
    })
    export default class BucketArea extends Vue {
        public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;

        public mounted(): void {
            this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, 1);
        }

        public get totalCountOfBuckets(): number {
            return this.$store.state.bucketUsageModule.totalCount;
        }

        public get buckets(): BucketUsage[] {
            return this.$store.state.bucketUsageModule.page.bucketUsages;
        }
        
        public get pages(): number {
            return this.$store.state.bucketUsageModule.page.pageCount;
        }
        
        public get search(): string {
            return this.$store.state.bucketUsageModule.cursor.search;
        }
    }
</script>

<style scoped lang="scss">
    .buckets-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 44px 40px 0 92px;
        
        p {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #384B65;
            margin-right: 50px;
            margin-block-start: 0em;
            margin-block-end: 0em;
        }
    }
    
    .table-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 20px 90px 0 40px;
    
        &:last-child {
            padding-left: 20px;
        }
    }
    
    .buckets-container {
        padding: 0px 40px 0 60px;
        
        table {
            width:98.5%;
            margin-top:20px;
        }
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
