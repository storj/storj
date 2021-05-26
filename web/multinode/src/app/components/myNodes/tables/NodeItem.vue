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
        <th>{{ node.earned | centsToDollars }}</th>
        <th>{{ node.version }}</th>
        <th :class="node.status">{{ node.status }}</th>
        <th class="overflow-visible">
            <node-options :id="node.id" />
        </th>
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

import { Node } from '@/nodes';

@Component({
    components: {
        NodeOptions,
    },
})
export default class NodeItem extends Vue {
    @Prop({default: () => new Node()})
    public node: Node;

    public get isSatelliteSelected(): boolean {
        return !!this.$store.state.nodes.selectedSatellite;
    }
}
</script>
