// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table v-if="nodes.length">
        <thead slot="head">
            <tr>
                <th class="align-left">NODE</th>
                <template v-if="isSatelliteSelected">
                    <th>SUSPENSION</th>
                    <th>AUDIT</th>
                    <th>UPTIME</th>
                </template>
                <template v-else>
                    <th>DISK SPACE USED</th>
                    <th>DISK SPACE LEFT</th>
                    <th>BANDWIDTH USED</th>
                </template>
                <th>EARNED</th>
                <th>VERSION</th>
                <th>STATUS</th>
                <th></th>
            </tr>
        </thead>
        <tbody slot="body">
            <node-item v-for="node in nodes" :key="node.id" :node="node" />
        </tbody>
    </base-table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BaseTable from '@/app/components/common/BaseTable.vue';
import NodeItem from '@/app/components/myNodes/tables/NodeItem.vue';

import { Node } from '@/nodes';

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
}
</script>
