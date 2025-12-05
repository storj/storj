// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="update-name">
        <div class="update-name__button" @click.stop="openModal">Update Name</div>
        <v-modal v-if="isModalShown" @onClose="closeModal">
            <h2 slot="header">Set name for node</h2>
            <div slot="body" class="update-name__body">
                <div class="update-name__body__node-id-container">
                    <span>{{ nodeId }}</span>
                </div>
                <headered-input
                    class="update-name__body__input"
                    label="Displayed name"
                    placeholder="Name"
                    :error="nameError"
                    @setData="setNodeName"
                />
            </div>
            <div slot="footer" class="delete-node__footer">
                <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                <v-button label="Set Name" width="205px" :on-press="onSetName" />
            </div>
        </v-modal>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { UpdateNodeModel } from '@/nodes';
import { Notify } from '@/app/plugins';

import HeaderedInput from '@/app/components/common/HeaderedInput.vue';
import VButton from '@/app/components/common/VButton.vue';
import VModal from '@/app/components/common/VModal.vue';

// @vue/component
@Component({
    components: {
        VButton,
        HeaderedInput,
        VModal,
    },
})
export default class AddNewNode extends Vue {
    @Prop({ default: '' })
    public nodeId: string;

    public nodeName = '';
    private nameError = '';
    public isModalShown = false;
    public notify = new Notify();

    private isLoading = false;

    /**
     * Sets node name field from value string.
     */
    public setNodeName(value: string): void {
        this.nodeName = value.trim();
        this.nameError = '';
    }

    public openModal(): void {
        this.isModalShown = true;
    }

    public closeModal(): void {
        this.isLoading = false;
        this.isModalShown = false;
        this.$emit('closeOptions');
    }

    public async onSetName(): Promise<void> {
        if (this.isLoading) { return; }

        if (!this.nodeName) {
            this.nameError = 'This field is required. Please enter a valid node name';

            return;
        }

        this.isLoading = true;

        try {
            await this.$store.dispatch('nodes/updateName', new UpdateNodeModel(this.nodeId, this.nodeName));
            this.notify.info({ message: 'Node name updated successfully.' });
            this.closeModal();
        } catch (error) {
            console.error(error);
            this.notify.error({ message: error.message, title: error?.name });
            this.isLoading = false;
        }
    }
}
</script>

<style lang="scss">
    .update-name {

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
            width: 441px;

            &__node-id-container {
                width: 100%;
                box-sizing: border-box;
                padding: 10px 12px;
                font-family: 'font_regular', sans-serif;
                font-size: 13px;
                color: var(--v-header-base);
                background: var(--v-active-base);
                border-radius: 32px;
                text-align: center;
            }

            &__input {
                margin-top: 42px !important;
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

