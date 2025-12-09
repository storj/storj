// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="update-name">
        <div class="update-name__button" @click.stop="openModal">Update Name</div>
        <v-modal v-if="isModalShown" @on-close="closeModal">
            <template #header>
                <h2>Set name for node</h2>
            </template>
            <template #body>
                <div class="update-name__body">
                    <div class="update-name__body__node-id-container">
                        <span>{{ nodeId }}</span>
                    </div>
                    <headered-input
                        class="update-name__body__input"
                        label="Displayed name"
                        placeholder="Name"
                        :error="nameError"
                        @set-data="setNodeName"
                    />
                </div>
            </template>
            <template #footer>
                <div class="delete-node__footer">
                    <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                    <v-button label="Set Name" width="205px" :on-press="onSetName" />
                </div>
            </template>
        </v-modal>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { UpdateNodeModel } from '@/nodes';
import { useNodesStore } from '@/app/store/nodesStore';

import HeaderedInput from '@/app/components/common/HeaderedInput.vue';
import VButton from '@/app/components/common/VButton.vue';
import VModal from '@/app/components/common/VModal.vue';

const nodesStore = useNodesStore();

const props = withDefaults(defineProps<{
    nodeId?: string;
}>(), {
    nodeId: '',
});

const emit = defineEmits<{
    (e: 'closeOptions'): void;
}>();

const nodeName = ref<string>('');
const nameError = ref<string>('');
const isModalShown = ref<boolean>(false);
const isLoading = ref<boolean>(false);

function setNodeName(value: string): void {
    nodeName.value = value.trim();
    nameError.value = '';
}

function openModal(): void {
    isModalShown.value = true;
}

function closeModal(): void {
    isLoading.value = false;
    isModalShown.value = false;
    emit('closeOptions');
}

async function onSetName(): Promise<void> {
    if (isLoading.value) return;

    if (!nodeName.value) {
        nameError.value = 'This field is required. Please enter a valid node name';
        return;
    }

    isLoading.value = true;

    try {
        await nodesStore.updateName(new UpdateNodeModel(props.nodeId, nodeName.value));
        closeModal();
    } catch (error) {
        console.error(error);
        isLoading.value = false;
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

