// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="table-item">
        <th class="align-left">{{ node.displayedName }}</th>
        <template v-if="isSatelliteSelected">
            <th>{{ node.suspensionScore | floatToPercentage }}</th>
            <th>{{ node.auditScore | floatToPercentage }}</th>
            <th>{{ node.onlineScore | floatToPercentage }}</th>
        </template>
        <template v-else>
            <th>{{ node.diskSpaceUsed | bytesToBase10String }}</th>
            <th>{{ node.diskSpaceLeft | bytesToBase10String }}</th>
            <th>{{ node.bandwidthUsed | bytesToBase10String }}</th>
        </template>
        <th>{{ node.earnedCents | centsToDollars }}</th>
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
import { useStore } from '@/app/utils/composables';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

const store = useStore();

withDefaults(defineProps<{
    node: Node;
}>(), {
    node: () => new Node(),
});

const isSatelliteSelected = computed(() => !!store.state.nodes.selectedSatellite);
</script>
