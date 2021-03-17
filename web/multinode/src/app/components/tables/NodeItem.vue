// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="node-item">
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
            <div class="node-item__options-button" @click.stop="toggleOptions">
                <more-icon />
            </div>
            <div class="node-item__options" v-if="areOptionsShown" v-click-outside="closeOptions">
                <div @click.stop="() => onCopy(node.id)" class="node-item__options__item">Copy Node ID</div>
                <delete-node :node-id="node.id" />
                <update-name :node-id="node.id" />
            </div>
        </th>
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import DeleteNode from '@/app/components/modals/DeleteNode.vue';
import UpdateName from '@/app/components/modals/UpdateName.vue';

import MoreIcon from '@/../static/images/icons/more.svg';

import { Node } from '@/nodes';

@Component({
    components: {
        UpdateName,
        DeleteNode,
        MoreIcon,
    },
})
export default class NodeItem extends Vue {
    @Prop({default: () => new Node()})
    public node: Node;

    public areOptionsShown: boolean = false;

    public get isSatelliteSelected(): boolean {
        return !!this.$store.state.nodes.selectedSatellite;
    }

    public toggleOptions(): void {
        this.areOptionsShown = !this.areOptionsShown;
    }

    public closeOptions(): void {
        if (!this.areOptionsShown) return;

        this.areOptionsShown = false;
    }

    public async onCopy(id: string): Promise<void> {
        try {
            await this.$copyText(id);
        } catch (error) {
            console.error(error.message);
        }

        this.closeOptions();
    }
}
</script>

<style scoped lang="scss">
    .node-item {
        height: 56px;
        text-align: right;
        font-size: 16px;
        color: var(--c-line);

        th {
            box-sizing: border-box;
            padding: 0 20px;
            max-width: 250px;
            white-space: nowrap;
            text-overflow: ellipsis;
            position: relative;
            overflow: hidden;
        }

        &:nth-of-type(even) {
            background: var(--c-block-gray);
        }

        th:not(:first-of-type) {
            font-family: 'font_medium', sans-serif;
        }

        &__options-button {
            width: 24px;
            height: 24px;
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            position: relative;

            &:hover {
                background: var(--c-background);
            }
        }

        &__options {
            position: absolute;
            top: 16px;
            right: 55px;
            width: 140px;
            height: auto;
            background: white;
            border-radius: var(--br-table);
            font-family: 'font_medium', sans-serif;
            border: 1px solid var(--c-gray--light);
            font-size: 14px;
            color: var(--c-title);
            z-index: 999;

            &__item {
                box-sizing: border-box;
                padding: 16px;
                cursor: pointer;
                text-align: left;

                &:hover {
                    background: var(--c-background);
                }
            }
        }
    }

    .online {
        color: var(--c-success);
    }

    .offline {
        color: var(--c-error);
    }

    .align-left {
        text-align: left;
    }

    .overflow-visible {
        overflow: visible !important;
    }
</style>
