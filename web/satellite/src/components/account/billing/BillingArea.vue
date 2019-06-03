// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div>
		<div v-if="billing > 0" class="billing-overflow">
			<div class="billing-container">
				<table class="billing-container__table">
					<SortingHeader />
					<BillingItem />
				</table>
				<PaginationArea />
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import BillingItem from '@/components/account/billing/BillingItem.vue';
    import PaginationArea from '@/components/account/billing/PaginationArea.vue';
    import SortingHeader from '@/components/account/billing/SortingHeader.vue';

    @Component({
        data: function () {
            return {
                billing: 1,
            };
        },
        components: {
            EmptyState,
            SortingHeader,
            BillingItem,
            PaginationArea,
        }
    })
    export default class BillingArea extends Vue {}
</script>

<style scoped lang="scss">
	.billing-container {
		margin-top: 83px;

		&__table {
			margin-top: 20px;
			width: 100%;
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

	@media screen and (max-height: 860px) {
		.billing-overflow {
			overflow-y: scroll;
			height: 700px;
		}
	}

	@media screen and (max-height: 835px) {
		.billing-overflow {
			overflow-y: scroll;
			height: 660px;
		}
	}

	@media screen and (max-height: 815px) {
		.billing-overflow {
			overflow-y: scroll;
			height: 660px;
		}
	}

	@media screen and (max-height: 795px) {
		.billing-overflow {
			overflow-y: scroll;
			height: 640px;
		}
	}

	@media screen and (max-height: 780px) {
		.billing-overflow {
			overflow-y: scroll;
			height: 620px;
		}
	}
</style>
