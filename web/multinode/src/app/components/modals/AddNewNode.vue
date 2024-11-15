// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-new-node">
        <v-button :with-plus="true" label="New Node" :on-press="openModal" width="152px" />
        <v-modal v-if="isAddNewNodeModalShown" @onClose="closeModal">
            <h2 slot="header">Add New Node</h2>
            <div slot="body" class="add-new-node__body">
                <headered-input
                    class="add-new-node__body__input"
                    label="Node ID"
                    placeholder="Enter Node ID"
                    :error="idError"
                    @setData="setNodeId"
                />
                <headered-input
                    class="add-new-node__body__input"
                    label="Node Name"
                    placeholder="Enter Node Name"
                    :error="nameError"
                    @setData="setNodeName"
                />
                <headered-input
                    class="add-new-node__body__input"
                    label="Public IP Address"
                    placeholder="Enter Public IP Address and Port"
                    :error="publicIPError"
                    @setData="setPublicIP"
                />
                <headered-input
                    class="add-new-node__body__input"
                    label="API Key"
                    placeholder="Enter API Key"
                    :error="apiKeyError"
                    @setData="setApiKey"
                />
            </div>
            <div slot="footer" class="add-new-node__footer">
                <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                <v-button label="Create" width="205px" :on-press="onCreate" />
            </div>
        </v-modal>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { CreateNodeFields } from '@/nodes';
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
    public isAddNewNodeModalShown = false;
    private nodeToAdd: CreateNodeFields = new CreateNodeFields();

    private isLoading = false;
    // errors
    private idError = '';
    private publicIPError = '';
    private apiKeyError = '';
    private nameError = '';
    public notify = new Notify();

    public async openModal(): Promise<void> {
        this.isAddNewNodeModalShown = true;
    }

    public closeModal(): void {
        this.nodeToAdd = new CreateNodeFields();
        this.idError = '';
        this.publicIPError = '';
        this.apiKeyError = '';
        this.nameError = '';
        this.isLoading = false;
        this.isAddNewNodeModalShown = false;
    }

    /**
     * Sets node id field from value string.
     */
    public setNodeId(value: string): void {
        this.nodeToAdd.id = value.trim();
        this.idError = '';
    }

    /**
     * Sets node public ip field from value string.
     */
    public setPublicIP(value: string): void {
        this.nodeToAdd.publicAddress = value.trim();
        this.publicIPError = '';
    }

    /**
     * Sets API key field from value string.
     */
    public setApiKey(value: string): void {
        this.nodeToAdd.apiSecret = value.trim();
        this.apiKeyError = '';
    }

    /**
     * Sets node name field from value string.
     */
    public setNodeName(value: string): void {
        this.nodeToAdd.name = value.trim();
        this.nameError = '';
    }

    public async onCreate(): Promise<void> {
        if (this.isLoading) { return; }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }

        try {
            await this.$store.dispatch('nodes/add', this.nodeToAdd);
            this.notify.success({ message: 'Node Added Successfully' });
        } catch (error) {
            console.error(error);
            this.notify.error({ message: error.message, title: error?.name });
            this.isLoading = false;
        }

        this.closeModal();
    }

    private validateFields(): boolean {
        let hasNoErrors = true;

        if (!this.nodeToAdd.id) {
            this.idError = 'This field is required. Please enter a valid node ID';
            hasNoErrors = false;
        }

        if (!this.nodeToAdd.name) {
            this.nameError = 'This field is required. Please enter a valid node Name';
            hasNoErrors = false;
        }

        if (!this.nodeToAdd.publicAddress) {
            this.publicIPError = 'This field is required. Please enter a valid node Public Address';
            hasNoErrors = false;
        }

        if (!this.nodeToAdd.apiSecret) {
            this.apiKeyError = 'This field is required. Please enter a valid API Key';
            hasNoErrors = false;
        }

        return hasNoErrors;
    }
}
</script>

<style lang="scss">
    .add-new-node {

        h2 {
            margin: 0;
            font-size: 32px;
        }

        &__body {
            width: 441px;

            &__input:not(:first-of-type) {
                margin-top: 20px;
            }
        }

        &__footer {
            width: 441px;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
    }
</style>
