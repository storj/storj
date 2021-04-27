// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="payouts-summary-item">
        <th class="align-left node-name">{{ payoutsSummary.nodeName }}</th>
        <th>{{ payoutsSummary.held | centsToDollars }}</th>
        <th>{{ payoutsSummary.paid | centsToDollars }}</th>
        <th class="overflow-visible options">
            <node-options :id="payoutsSummary.nodeID" />
        </th>
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

import { NodePayoutsSummary } from '@/payouts';

@Component({
    components: { NodeOptions },
})
export default class PayoutsSummaryItem extends Vue {
    @Prop({default: () => new NodePayoutsSummary()})
    public payoutsSummary: NodePayoutsSummary;
}
</script>

<style scoped lang="scss">
    .payouts-summary-item {
        height: 56px;
        text-align: right;
        font-size: 16px;
        color: var(--c-line);
        cursor: pointer;

        th {
            box-sizing: border-box;
            padding: 0 20px;
            max-width: 250px;
            white-space: nowrap;
            text-overflow: ellipsis;
            position: relative;
            overflow: hidden;
        }

        &:nth-of-type(even) {
            background: var(--c-block-gray);
        }

        th:not(:first-of-type) {
            font-family: 'font_medium', sans-serif;
        }

        .node-name {
            color: var(--c-primary);
        }
    }

    .align-left {
        text-align: left;
    }

    .overflow-visible {
        overflow: visible !important;
    }

    .options {
        width: 60px;
        text-align: center;
    }
</style>
