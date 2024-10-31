// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-node">
        <div class="delete-node__button" @click.stop="openModal">Delete Node</div>
        <v-modal v-if="isModalShown" @onClose="closeModal">
            <h2 slot="header">Delete this node?</h2>
            <div slot="body" class="delete-node__body">
                <div class="delete-node__body__node-id-container">
                    <span>{{ nodeId }}</span>
                </div>
            </div>
            <div slot="footer" class="delete-node__footer">
                <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                <v-button label="Delete" :is-deletion="true" width="205px" :on-press="onDelete" />
            </div>
        </v-modal>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Notify } from '@/app/plugins';

import VButton from '@/app/components/common/VButton.vue';
import VModal from '@/app/components/common/VModal.vue';

// @vue/component
@Component({
    components: {
        VButton,
        VModal,
    },
})
export default class AddNewNode extends Vue {
    @Prop({ default: '' })
    public nodeId: string;

    public isModalShown = false;
    public notify = new Notify();

    private isLoading = false;

    public openModal(): void {
        this.isModalShown = true;
    }

    public closeModal(): void {
        this.isLoading = false;
        this.isModalShown = false;
        this.$emit('closeOptions');
    }

    public async onDelete(): Promise<void> {
        if (this.isLoading) { return; }

        this.isLoading = true;

        try {
            await this.$store.dispatch('nodes/delete', this.nodeId);
            this.notify.info({ message: 'Node Deleted Successfully' });
            this.closeModal();
        } catch (error) {
            console.error(error);
            this.isLoading = false;
        }
    }
}
</script>

<style lang="scss">
    .delete-node {

        h2 {
            margin: 0;
            font-size: 32px;
        }

        &__button {
            width: 100%;
            box-sizing: border-box;
            padding: 16px;
            cursor: pointer;
            text-align: left;
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            color: var(--v-header-base);

            &:hover {
                background: var(--v-active-base);
            }
        }

        &__body {
            width: 460px;

            &__node-id-container {
                width: 100%;
                box-sizing: border-box;
                padding: 10px 12px;
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--v-header-base);
                background: var(--v-active-base);
                border-radius: 32px;
            }
        }

        &__footer {
            width: 460px;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
    }
</style>
