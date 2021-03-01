// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table class="nodes-table" v-if="nodes.length" border="0" cellpadding="0" cellspacing="0">
        <thead>
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
        <tbody>
            <node-item v-for="node in nodes" :key="node.id" :node="node" />
        </tbody>
    </table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NodeItem from '@/app/components/tables/NodeItem.vue';

import { Node } from '@/nodes';

@Component({
    components: {
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

<style scoped lang="scss">
    .nodes-table {
        width: 100%;
        border: 1px solid var(--c-gray--light);
        border-radius: var(--br-table);
        font-family: 'font_semiBold', sans-serif;

        th {
            box-sizing: border-box;
            padding: 0 20px;
            max-width: 250px;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
        }

        thead {
            background: var(--c-block-gray);

            tr {
                height: 40px;
                font-size: 12px;
                color: var(--c-gray);
                border-radius: var(--br-table);
                text-align: right;
            }
        }

        .align-left {
            text-align: left;
        }
    }
</style>
