// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div>
		<div v-if="buckets > 0" class="api-keys-header">
			<p>Buckets</p>
			<SearchArea/>
		</div>
		<div v-if="buckets > 0" class="api-keys-container">
			<div class="api-keys-container__content">
				<BucketItem />
				<BucketItem />
				<BucketItem />
				<BucketItem />
				<BucketItem />
				<BucketItem />
			</div>
			<PaginationArea />
		</div>
		<EmptyState
			v-if="buckets === 0"
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
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

    @Component({
        data: function () {
            return {
                emptyImage: EMPTY_STATE_IMAGES.API_KEY,
                buckets: 1,
            };
        },
        components: {
            EmptyState,
            SearchArea,
            BucketItem,
            PaginationArea,
        }
    })
    export default class BucketArea extends Vue {}
</script>

<style scoped lang="scss">
	.api-keys-header {
		padding: 44px 40px 0 40px;
	}
	.api-keys-container {
		padding: 0px 40px 0 60px;
	}
</style>
