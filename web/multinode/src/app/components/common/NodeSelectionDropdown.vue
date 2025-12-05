// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="nodes" />
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { Node } from '@/nodes';

import VDropdown, { Option } from '@/app/components/common/VDropdown.vue';

// @vue/component
@Component({
    components: { VDropdown },
})
export default class NodeSelectionDropdown extends Vue {
    /**
     * List of nodes from store.
     */
    public get nodes(): Option[] {
        const nodes: Node[] = this.$store.state.nodes.nodes;

        const options: Option[] = nodes.map(
            (node: Node) => new Option(node.displayedName, () => this.onNodeClick(node.id)),
        );

        return [new Option('All Nodes', () => this.onNodeClick()), ...options];
    }

    /**
     * Callback for node click.
     * @param nodeId - node id to select
     */
    public async onNodeClick(nodeId = ''): Promise<void> {
        await this.$store.dispatch('nodes/selectNode', nodeId);
    }
}
</script>
