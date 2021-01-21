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
            </tr>
        </thead>
        <tbody>
            <tr v-for="node in nodes" :key="node.id">
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
            </tr>
        </tbody>
    </table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { Node } from '@/nodes';

@Component
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
        overflow: hidden;

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

        tbody {

            tr {
                height: 56px;
                text-align: right;
                font-size: 16px;
                color: var(--c-line);

                &:nth-of-type(even) {
                    background: var(--c-block-gray);
                }

                th:not(:first-of-type) {
                    font-family: 'font_medium', sans-serif;
                }
            }

            .online {
                color: var(--c-success);
            }

            .offline {
                color: var(--c-error);
            }
        }

        .align-left {
            text-align: left;
        }
    }
</style>
