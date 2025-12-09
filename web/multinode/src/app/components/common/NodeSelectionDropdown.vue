// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="nodes" />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Node } from '@/nodes';
import { Option } from '@/app/types/common';
import { useNodesStore } from '@/app/store/nodesStore';

import VDropdown from '@/app/components/common/VDropdown.vue';

const nodesStore = useNodesStore();

const nodes = computed<Option[]>(() => {
    const nodeList: Node[] = nodesStore.state.nodes;

    const options: Option[] = nodeList.map(
        (node: Node) => new Option(node.displayedName, () => onNodeClick(node.id)),
    );

    return [new Option('All Nodes', () => onNodeClick()), ...options];
});

async function onNodeClick(nodeId = ''): Promise<void> {
    await nodesStore.selectNode(nodeId);
}
</script>
