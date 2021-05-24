// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table class="payouts-summary-table" v-if="nodePayoutsSummary.length" border="0" cellpadding="0" cellspacing="0">
        <thead>
        <tr>
            <th class="align-left">NODE</th>
            <th>HELD</th>
            <th>PAID</th>
            <th class="options"></th>
        </tr>
        </thead>
        <tbody>
            <payouts-summary-item v-for="payoutSummary in nodePayoutsSummary" :key="payoutSummary.nodeId" :payouts-summary="payoutSummary"/>
        </tbody>
    </table>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import NodeItem from '@/app/components/myNodes/tables/NodeItem.vue';
import PayoutsSummaryItem from '@/app/components/payouts/tables/PayoutsSummaryItem.vue';

import { NodePayoutsSummary } from '@/payouts';

@Component({
    components: {
        PayoutsSummaryItem,
        NodeItem,
    },
})
export default class PayoutsSummaryTable extends Vue {
    @Prop({default: () => []})
    public nodePayoutsSummary: NodePayoutsSummary[];
}
</script>

<style scoped lang="scss">
    .payouts-summary-table {
        width: 100%;
        border: 1px solid var(--c-gray--light);
        border-radius: var(--br-table);
        font-family: 'font_semiBold', sans-serif;
        z-index: 999;

        th {
            box-sizing: border-box;
            padding: 0 20px;
            max-width: 250px;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
        }

        thead {
            background: var(--c-block-gray);

            tr {
                height: 40px;
                font-size: 12px;
                color: var(--c-gray);
                border-radius: var(--br-table);
                text-align: right;
            }
        }

        .align-left {
            text-align: left;
        }

        .options {
            width: 60px;
        }
    }
</style>
