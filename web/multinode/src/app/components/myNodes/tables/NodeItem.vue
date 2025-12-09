// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="table-item">
        <th class="align-left">{{ node.displayedName }}</th>
        <template v-if="isSatelliteSelected">
            <th>{{ Percentage.fromFloat(node.suspensionScore) }}</th>
            <th>{{ Percentage.fromFloat(node.auditScore) }}</th>
            <th>{{ Percentage.fromFloat(node.onlineScore) }}</th>
        </template>
        <template v-else>
            <th>{{ Size.toBase10String(node.diskSpaceUsed) }}</th>
            <th>{{ Size.toBase10String(node.diskSpaceLeft) }}</th>
            <th>{{ Size.toBase10String(node.bandwidthUsed) }}</th>
        </template>
        <th>{{ Currency.dollarsFromCents(node.earnedCents) }}</th>
        <th>{{ node.version }}</th>
        <th :class="node.status">{{ node.status }}</th>
        <th class="overflow-visible">
            <node-options :id="node.id" />
        </th>
    </tr>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Node } from '@/nodes';
import { Currency } from '@/app/utils/currency';
import { Size } from '@/private/memory/size';
import { Percentage } from '@/app/utils/percentage';
import { useNodesStore } from '@/app/store/nodesStore';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

const nodesStore = useNodesStore();

withDefaults(defineProps<{
    node?: Node;
}>(), {
    node: () => new Node(),
});

const isSatelliteSelected = computed(() => !!nodesStore.state.selectedSatellite);
</script>
