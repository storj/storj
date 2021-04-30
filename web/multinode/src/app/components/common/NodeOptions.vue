// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="options-button" @click.stop="toggleOptions">
        <more-icon />
        <div class="options" v-if="areOptionsShown" v-click-outside="closeOptions">
            <div @click.stop="onCopy" class="options__item">Copy Node ID</div>
            <delete-node :node-id="id" />
            <update-name :node-id="id" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import DeleteNode from '@/app/components/modals/DeleteNode.vue';
import UpdateName from '@/app/components/modals/UpdateName.vue';

import MoreIcon from '@/../static/images/icons/more.svg';

@Component({
    components: {
        UpdateName,
        DeleteNode,
        MoreIcon,
    },
})
export default class NodeOptions extends Vue {
    @Prop({default: ''})
    public id: string;

    public areOptionsShown: boolean = false;

    public toggleOptions(): void {
        this.areOptionsShown = !this.areOptionsShown;
    }

    public closeOptions(): void {
        if (!this.areOptionsShown) return;

        this.areOptionsShown = false;
    }

    /**
     * Copies node id to clipboard and closes popup.
     */
    public async onCopy(): Promise<void> {
        try {
            await this.$copyText(this.id);
        } catch (error) {
            console.error(error.message);
        }

        this.closeOptions();
    }
}
</script>

<style scoped lang="scss">
    .options-button {
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

    .options {
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
</style>
