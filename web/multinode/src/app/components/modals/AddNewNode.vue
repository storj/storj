// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-new-node">
        <v-button :with-plus="true" label="New Node" :on-press="openModal" width="152px" />
        <v-modal v-if="isAddNewNodeModalShown" @on-close="closeModal">
            <template #header>
                <h2>Add New Node</h2>
            </template>
            <template #body>
                <div class="add-new-node__body">
                    <headered-input
                        class="add-new-node__body__input"
                        label="Node ID"
                        placeholder="Enter Node ID"
                        :error="idError"
                        @set-data="setNodeId"
                    />
                    <headered-input
                        class="add-new-node__body__input"
                        label="Node Name"
                        placeholder="Enter Node Name"
                        :error="nameError"
                        @set-data="setNodeName"
                    />
                    <headered-input
                        class="add-new-node__body__input"
                        label="Public IP Address"
                        placeholder="Enter Public IP Address and Port"
                        :error="publicIPError"
                        @set-data="setPublicIP"
                    />
                    <headered-input
                        class="add-new-node__body__input"
                        label="API Key"
                        placeholder="Enter API Key"
                        :error="apiKeyError"
                        @set-data="setApiKey"
                    />
                </div>
            </template>
            <template #footer>
                <div class="add-new-node__footer">
                    <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                    <v-button label="Create" width="205px" :on-press="onCreate" />
                </div>
            </template>
        </v-modal>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { CreateNodeFields } from '@/nodes';
import { Notify } from '@/app/plugins';
import { useNodesStore } from '@/app/store/nodesStore';

import HeaderedInput from '@/app/components/common/HeaderedInput.vue';
import VButton from '@/app/components/common/VButton.vue';
import VModal from '@/app/components/common/VModal.vue';

const notify = new Notify();

const nodesStore = useNodesStore();

const isAddNewNodeModalShown = ref<boolean>(false);
const nodeToAdd = ref<CreateNodeFields>(new CreateNodeFields());
const isLoading = ref<boolean>(false);

const idError = ref<string>('');
const publicIPError = ref<string>('');
const apiKeyError = ref<string>('');
const nameError = ref<string>('');

function openModal(): void {
    isAddNewNodeModalShown.value = true;
}

function closeModal(): void {
    nodeToAdd.value = new CreateNodeFields();
    idError.value = '';
    publicIPError.value = '';
    apiKeyError.value = '';
    nameError.value = '';
    isLoading.value = false;
    isAddNewNodeModalShown.value = false;
}

function setNodeId(value: string): void {
    nodeToAdd.value.id = value.trim();
    idError.value = '';
}

function setPublicIP(value: string): void {
    nodeToAdd.value.publicAddress = value.trim();
    publicIPError.value = '';
}

function setApiKey(value: string): void {
    nodeToAdd.value.apiSecret = value.trim();
    apiKeyError.value = '';
}

function setNodeName(value: string): void {
    nodeToAdd.value.name = value.trim();
    nameError.value = '';
}

async function onCreate(): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;

    if (!validateFields()) {
        isLoading.value = false;
        return;
    }

    try {
        await nodesStore.add(nodeToAdd.value);
        notify.success({ message: 'Node Added Successfully' });
    } catch (error) {
        console.error(error);
        isLoading.value = false;
    }

    closeModal();
}

function validateFields(): boolean {
    let hasNoErrors = true;

    if (!nodeToAdd.value.id) {
        idError.value = 'This field is required. Please enter a valid node ID';
        hasNoErrors = false;
    }

    if (!nodeToAdd.value.name) {
        nameError.value = 'This field is required. Please enter a valid node Name';
        hasNoErrors = false;
    }

    if (!nodeToAdd.value.publicAddress) {
        publicIPError.value = 'This field is required. Please enter a valid node Public Address';
        hasNoErrors = false;
    }

    if (!nodeToAdd.value.apiSecret) {
        apiKeyError.value = 'This field is required. Please enter a valid API Key';
        hasNoErrors = false;
    }

    return hasNoErrors;
}
</script>

<style scoped lang="scss">
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
