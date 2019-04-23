// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div>
		<div v-if="buckets.length > 0" class="buckets-overflow">
			<div class="buckets-header">
				<p>Buckets</p>
				<SearchArea/>
			</div>
			<div class="buckets-container">
				<table style="width:98.5%; margin-top:20px;">
					<SortingHeader />
                    <BucketItem v-for="bucket in buckets" v-bind:bucket="bucket" />
					<BucketItem v-for="bucket in buckets" v-bind:bucket="bucket" />
				</table>
				<PaginationArea />
			</div>
		</div>
		<EmptyState
			v-if="pages.length === 0"
			mainTitle="You have no Buckets yet"
			:imageSource="emptyImage" />
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import SearchArea from '@/components/buckets/SearchArea.vue';
    import BucketItem from '@/components/buckets/BucketItem.vue';
    import PaginationArea from '@/components/buckets/PaginationArea.vue';
    import SortingHeader from '@/components/buckets/SortingHeader.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
	import { BUCKET_USAGE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
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
        },
		computed: {
        	buckets: function () {
				return this.$store.state.bucketUsageModule.currentPage.bucketUsages;
			},
			pages: function () {
        		return this.$store.state.bucketUsageModule.pages;
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
	}

	@media screen and (max-height: 880px) {
		.buckets-overflow {
			overflow-y: scroll;
			height: 600px;
		}
	}

	@media screen and (max-height: 700px) {
		.buckets-overflow {
			height: 570px;
		}
	}
</style>
