// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="nodes" />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Node } from '@/nodes';
import { useStore } from '@/app/utils/composables';
import { Option } from '@/app/types/common';

import VDropdown from '@/app/components/common/VDropdown.vue';

const store = useStore();

const nodes = computed<Option[]>(() => {
    const nodeList: Node[] = store.state.nodes.nodes;

    const options: Option[] = nodeList.map(
        (node: Node) => new Option(node.displayedName, () => onNodeClick(node.id)),
    );

    return [new Option('All Nodes', () => onNodeClick()), ...options];
});

async function onNodeClick(nodeId = ''): Promise<void> {
    await store.dispatch('nodes/selectNode', nodeId);
}
</script>
