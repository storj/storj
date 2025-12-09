// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="table-item payouts-summary-item" @click.prevent="redirectToByNodePayoutsPage">
        <th class="align-left node-name">{{ payoutsSummary.nodeName || payoutsSummary.nodeId }}</th>
        <th>{{ Currency.dollarsFromCents(payoutsSummary.held) }}</th>
        <th>{{ Currency.dollarsFromCents(payoutsSummary.paid) }}</th>
        <th class="overflow-visible options">
            <node-options :id="payoutsSummary.nodeId" />
        </th>
    </tr>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';

import { Config as RouterConfig } from '@/app/router';
import { NodePayoutsSummary } from '@/payouts';
import { Currency } from '@/app/utils/currency';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

const router = useRouter();

const props = defineProps<{
    payoutsSummary: NodePayoutsSummary;
}>();

function redirectToByNodePayoutsPage(): void {
    router.push({
        name: RouterConfig.PayoutsByNode.name,
        params: { id: props.payoutsSummary.nodeId },
    });
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
