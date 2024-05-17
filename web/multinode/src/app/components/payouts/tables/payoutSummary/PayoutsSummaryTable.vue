// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table v-if="nodePayoutsSummary.length">
        <thead slot="head">
            <tr>
                <th @click="sortBy('nodeName')" class="align-left">NODE{{sortByKey === 'nodeName' ? sortArrow : ''}}</th>
                <th @click="sortBy('held')">HELD{{sortByKey === 'held' ? sortArrow : ''}}</th>
                <th @click="sortBy('paid')">PAID{{sortByKey === 'paid' ? sortArrow : ''}}</th>
                <th class="options" />
            </tr>
        </thead>
        <tbody slot="body">
            <payouts-summary-item v-for="payoutSummary in sortedNodePayoutsSummary" :key="payoutSummary.nodeId" :payouts-summary="payoutSummary" />
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

    // Initialize sorting variables
    sortByKey: string = "";
    sortDirection: string = 'asc';

    // Cache the sort state in browser to persist between sessions
    created() {
        const savedSortByKey = localStorage.getItem('payoutSortByKey');
        const savedSortDirection = localStorage.getItem('payoutSortDirection');
        if (savedSortByKey) {
            this.sortByKey = savedSortByKey;
        }
        if (savedSortDirection) {
            this.sortDirection = savedSortDirection;
        }
    }

    // Sorted nodePayoutsSummary getter
    public get sortedNodePayoutsSummary(): NodePayoutsSummary[] {
        const key = this.sortByKey;
        const direction = this.sortDirection === 'asc' ? 1 : -1;
        if (key === "") return this.nodePayoutsSummary;
        return this.nodePayoutsSummary.slice().sort((a, b) => {
            if (a[key] < b[key]) return -direction;
            if (a[key] > b[key]) return direction;
            return 0;
        });
    }

    // Update sorting key and direction
    public sortBy(key: string) {
        if (this.sortByKey === key) {
            if (this.sortDirection === "asc") {
                this.sortDirection = "desc";
            } else {
                // Disable sorting after three clicks (flow: asc -> desc -> disable -> asc -> ...)
                this.sortByKey = "";
            }
        } else {
            this.sortByKey = key;
            this.sortDirection = 'asc';
        }

        localStorage.setItem('payoutSortByKey', this.sortByKey);
        localStorage.setItem('payoutSortDirection', this.sortDirection);
    }

    // Determine arrow icon
    public get sortArrow(): string {
        return this.sortDirection === 'asc' ? ' ↑' : ' ↓';
    }
}
</script>

<style scoped>
    th {
        user-select: none; /* Diable user selecting the headers for sort selection */
    }
</style>
