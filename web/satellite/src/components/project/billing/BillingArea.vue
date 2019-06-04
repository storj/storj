// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div>
		<div v-if="billing > 0" class="billing-overflow">
			<div class="billing-header">
				<p>Billing</p>
			</div>
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
    import BillingItem from '@/components/project/billing/BillingItem.vue';
    import PaginationArea from '@/components/project/billing/PaginationArea.vue';
    import SortingHeader from '@/components/project/billing/SortingHeader.vue';

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
	.billing-header {
		display: flex;
		align-items: center;
		justify-content: flex-start;

		p {
			font-family: 'font_bold';
			font-size: 24px;
			line-height: 29px;
			color: #384B65;
			margin-right: 50px;
			margin-block-start: 0.5em;
			margin-block-end: 0.5em;
		}
	}

	.billing-container {
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

	@media screen and (max-height: 880px) {
		.billing-overflow {
			overflow-y: scroll;
			height: 600px;
		}
	}

	@media screen and (max-height: 700px) {
		.billing-overflow {
			height: 570px;
		}
	}
</style>
