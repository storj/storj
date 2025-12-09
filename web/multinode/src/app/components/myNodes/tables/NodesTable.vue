// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table v-if="nodes.length">
        <template #head>
            <thead>
                <tr>
                    <th class="align-left" @click="sortBy('name')">NODE{{ sortByKey === 'name' ? sortArrow : '' }}</th>
                    <template v-if="isSatelliteSelected">
                        <th @click="sortBy('suspensionScore')">SUSPENSION{{ sortByKey === 'suspensionScore' ? sortArrow : '' }}</th>
                        <th @click="sortBy('auditScore')">AUDIT{{ sortByKey === 'auditScore' ? sortArrow : '' }}</th>
                        <th @click="sortBy('onlineScore')">UPTIME{{ sortByKey === 'onlineScore' ? sortArrow : '' }}</th>
                    </template>
                    <template v-else>
                        <th @click="sortBy('diskSpaceUsed')">DISK SPACE USED{{ sortByKey === 'diskSpaceUsed' ? sortArrow : '' }}</th>
                        <th @click="sortBy('diskSpaceLeft')">DISK SPACE LEFT{{ sortByKey === 'diskSpaceLeft' ? sortArrow : '' }}</th>
                        <th @click="sortBy('bandwidthUsed')">BANDWIDTH USED{{ sortByKey === 'bandwidthUsed' ? sortArrow : '' }}</th>
                    </template>
                    <th @click="sortBy('earned')">EARNED{{ sortByKey === 'earned' ? sortArrow : '' }}</th>
                    <th @click="sortBy('version')">VERSION{{ sortByKey === 'version' ? sortArrow : '' }}</th>
                    <th @click="sortBy('status')">STATUS{{ sortByKey === 'status' ? sortArrow : '' }}</th>
                    <th />
                </tr>
            </thead>
        </template>

        <template #body>
            <tbody>
                <node-item v-for="node in sortedNodes" :key="node.id" :node="node" />
            </tbody>
        </template>
    </base-table>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';

import { Node } from '@/nodes';
import { useNodesStore } from '@/app/store/nodesStore';

import BaseTable from '@/app/components/common/BaseTable.vue';
import NodeItem from '@/app/components/myNodes/tables/NodeItem.vue';

const nodesStore = useNodesStore();

const sortByKey = ref<string>('');
const sortDirection = ref<string>('asc');

const nodes = computed<Node[]>(() => nodesStore.state.nodes);
const isSatelliteSelected = computed<boolean>(() => !!nodesStore.state.selectedSatellite);
const sortArrow = computed<string>(() => sortDirection.value === 'asc' ? ' ↑' : ' ↓');
const sortedNodes = computed<Node[]>(() => {
    const key = sortByKey.value;
    const direction = sortDirection.value === 'asc' ? 1 : -1;
    if (key === '') return nodes.value;
    return nodes.value.slice().sort((a, b) => {
        if (a[key] < b[key]) return -direction;
        if (a[key] > b[key]) return direction;
        return 0;
    });
});

function sortBy(key: string): void {
    if (sortByKey.value === key) {
        if (sortDirection.value === 'asc') {
            sortDirection.value = 'desc';
        } else {
            // Disable sorting after three clicks (flow: asc -> desc -> disable -> asc -> ...)
            sortByKey.value = '';
        }
    } else {
        sortByKey.value = key;
        sortDirection.value = 'asc';
    }

    localStorage.setItem('nodesSortByKey', sortByKey.value);
    localStorage.setItem('nodesSortDirection', sortDirection.value);
}

onBeforeMount(() => {
    const savedSortByKey = localStorage.getItem('nodesSortByKey');
    const savedSortDirection = localStorage.getItem('nodesSortDirection');
    if (savedSortByKey) {
        sortByKey.value = savedSortByKey;
    }
    if (savedSortDirection) {
        sortDirection.value = savedSortDirection;
    }
});
</script>

<style scoped lang="scss">
    th {
        user-select: none; /* Diable user selecting the headers for sort selection */
    }
</style>
