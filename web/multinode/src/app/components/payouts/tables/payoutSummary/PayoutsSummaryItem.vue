// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="table-item payouts-summary-item" @click.prevent="redirectToByNodePayoutsPage">
        <th class="align-left node-name">{{ payoutsSummary.nodeName || payoutsSummary.nodeId }}</th>
        <th>{{ payoutsSummary.held | centsToDollars }}</th>
        <th>{{ payoutsSummary.paid | centsToDollars }}</th>
        <th class="overflow-visible options">
            <node-options :id="payoutsSummary.nodeId" />
        </th>
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Config as RouterConfig } from '@/app/router';
import { NodePayoutsSummary } from '@/payouts';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

// @vue/component
@Component({
    components: {
        NodeOptions,
    },
})
export default class PayoutsSummaryItem extends Vue {
    @Prop({ default: () => new NodePayoutsSummary() })
    public payoutsSummary: NodePayoutsSummary;

    public redirectToByNodePayoutsPage(): void {
        this.$router.push({
            name: RouterConfig.Payouts.with(RouterConfig.PayoutsByNode).name,
            params: { id: this.payoutsSummary.nodeId },
        });
    }
}
</script>

<style scoped lang="scss">
    .payouts-summary-item {
        cursor: pointer;

        .node-name {
            color: var(--c-primary);
        }
    }
</style>
