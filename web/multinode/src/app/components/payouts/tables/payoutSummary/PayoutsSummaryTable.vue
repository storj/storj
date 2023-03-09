// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table v-if="nodePayoutsSummary.length">
        <thead slot="head">
            <tr>
                <th class="align-left">NODE</th>
                <th>HELD</th>
                <th>PAID</th>
                <th class="options" />
            </tr>
        </thead>
        <tbody slot="body">
            <payouts-summary-item v-for="payoutSummary in nodePayoutsSummary" :key="payoutSummary.nodeId" :payouts-summary="payoutSummary" />
        </tbody>
    </base-table>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { NodePayoutsSummary } from '@/payouts';

import BaseTable from '@/app/components/common/BaseTable.vue';
import PayoutsSummaryItem from '@/app/components/payouts/tables/payoutSummary/PayoutsSummaryItem.vue';

// @vue/component
@Component({
    components: {
        BaseTable,
        PayoutsSummaryItem,
    },
})
export default class PayoutsSummaryTable extends Vue {
    @Prop({ default: () => [] })
    public nodePayoutsSummary: NodePayoutsSummary[];
}
</script>
