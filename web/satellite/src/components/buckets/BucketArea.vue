// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="buckets-overflow" v-if="pages !== 0">
            <div class="buckets-header">
                <p>Buckets</p>
                <SearchArea/>
            </div>
            <div v-if="buckets.length > 0" class="buckets-container">
                <table>
                    <SortingHeader />
                    <BucketItem v-for="(bucket, index) in buckets" v-bind:bucket="bucket" v-bind:key="index" />
                </table>
                <PaginationArea />
            </div>
            <EmptyState
                class="empty-container"
                v-if="pages === 0 && search && search.length > 0"
                mainTitle="Nothing found :("
                :imageSource="emptyImage" />
        </div>
        <NoBucketArea v-if="pages === 0 && !search" />
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
        mounted: function() {
            this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, 1)
        },
		data: function () {
			return {
				emptyImage: EMPTY_STATE_IMAGES.API_KEY
			};
		},
		components: {
			EmptyState,
			SearchArea,
			SortingHeader,
			BucketItem,
			PaginationArea,
			NoBucketArea,
		},
		computed: {
			buckets: function (): BucketUsage[] {
				return this.$store.state.bucketUsageModule.page.bucketUsages;
			},
			pages: function (): number {
				return this.$store.state.bucketUsageModule.page.pageCount;
			},
			search: function (): string {
				return this.$store.state.bucketUsageModule.cursor.search;
			}
		}
	})

	export default class BucketArea extends Vue {}
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
