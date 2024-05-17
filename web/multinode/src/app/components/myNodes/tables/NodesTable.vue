// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table v-if="nodes.length">
        <thead slot="head">
            <tr>
                <th class="align-left" @click="sortBy('name')">NODE{{sortByKey === 'name' ? sortArrow : ''}}</th>
                <template v-if="isSatelliteSelected">
                    <th @click="sortBy('suspensionScore')">SUSPENSION{{sortByKey === 'suspensionScore' ? sortArrow : ''}}</th>
                    <th @click="sortBy('auditScore')">AUDIT{{sortByKey === 'auditScore' ? sortArrow : ''}}</th>
                    <th @click="sortBy('onlineScore')">UPTIME{{sortByKey === 'onlineScore' ? sortArrow : ''}}</th>
                </template>
                <template v-else>
                    <th @click="sortBy('diskSpaceUsed')">DISK SPACE USED{{sortByKey === 'diskSpaceUsed' ? sortArrow : ''}}</th>
                    <th @click="sortBy('diskSpaceLeft')">DISK SPACE LEFT{{sortByKey === 'diskSpaceLeft' ? sortArrow : ''}}</th>
                    <th @click="sortBy('bandwidthUsed')">BANDWIDTH USED{{sortByKey === 'bandwidthUsed' ? sortArrow : ''}}</th>
                </template>
                <th @click="sortBy('earned')">EARNED{{sortByKey === 'earned' ? sortArrow : ''}}</th>
                <th @click="sortBy('version')">VERSION{{sortByKey === 'version' ? sortArrow : ''}}</th>
                <th @click="sortBy('status')">STATUS{{sortByKey === 'status' ? sortArrow : ''}}</th>
                <th />
            </tr>
        </thead>
        <tbody slot="body">
            <node-item v-for="node in sortedNodes" :key="node.id" :node="node" />
        </tbody>
    </base-table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { Node } from '@/nodes';

import BaseTable from '@/app/components/common/BaseTable.vue';
import NodeItem from '@/app/components/myNodes/tables/NodeItem.vue';

// @vue/component
@Component({
    components: {
        BaseTable,
        NodeItem,
    },
})
export default class NodesTable extends Vue {
    public get nodes(): Node[] {
        return this.$store.state.nodes.nodes;
    }

    public get isSatelliteSelected(): boolean {
        return !!this.$store.state.nodes.selectedSatellite;
    }

    // Initialize sorting variables
    sortByKey: string = "";
    sortDirection: string = 'asc';

    // Cache the sort state in browser to persist between sessions
    created() {
        const savedSortByKey = localStorage.getItem('nodesSortByKey');
        const savedSortDirection = localStorage.getItem('nodesSortDirection');
        if (savedSortByKey) {
            this.sortByKey = savedSortByKey;
        }
        if (savedSortDirection) {
            this.sortDirection = savedSortDirection;
        }
    }

    // Sorted nodes getter
    public get sortedNodes(): Node[] {
        const key = this.sortByKey;
        const direction = this.sortDirection === 'asc' ? 1 : -1;
        if (key === "") return this.nodes;
        return this.nodes.slice().sort((a, b) => {
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

        localStorage.setItem('nodesSortByKey', this.sortByKey);
        localStorage.setItem('nodesSortDirection', this.sortDirection);
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
