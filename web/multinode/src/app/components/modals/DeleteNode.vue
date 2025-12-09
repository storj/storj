// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-node">
        <div class="delete-node__button" @click.stop="openModal">Delete Node</div>
        <v-modal v-if="isModalShown" @on-close="closeModal">
            <template #header>
                <h2>Delete this node?</h2>
            </template>
            <template #body>
                <div class="delete-node__body">
                    <div class="delete-node__body__node-id-container">
                        <span>{{ nodeId }}</span>
                    </div>
                </div>
            </template>
            <template #footer>
                <div class="delete-node__footer">
                    <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                    <v-button label="Delete" :is-deletion="true" width="205px" :on-press="onDelete" />
                </div>
            </template>
        </v-modal>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useNodesStore } from '@/app/store/nodesStore';

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

const isModalShown = ref<boolean>(false);
const isLoading = ref<boolean>(false);

function openModal(): void {
    isModalShown.value = true;
}

function closeModal(): void {
    isLoading.value = false;
    isModalShown.value = false;
    emit('closeOptions');
}

async function onDelete(): Promise<void> {
    if (isLoading.value) { return; }

    isLoading.value = true;

    try {
        await nodesStore.deleteNode(props.nodeId);
        closeModal();
    } catch (error) {
        console.error(error);
        isLoading.value = false;
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
