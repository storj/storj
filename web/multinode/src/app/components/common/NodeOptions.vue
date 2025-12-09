// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="options-button" @click.stop="openOptions">
        <more-icon />
        <div v-if="areOptionsShown" v-click-outside="closeOptions" class="options">
            <div class="options__item" @click.stop="onCopy">Copy Node ID</div>
            <delete-node :node-id="id" @close-options="closeOptions" />
            <update-name :node-id="id" @close-options="closeOptions" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import DeleteNode from '@/app/components/modals/DeleteNode.vue';
import UpdateName from '@/app/components/modals/UpdateName.vue';

import MoreIcon from '@/../static/images/icons/more.svg';

const props = withDefaults(defineProps<{
    id?: string;
}>(), {
    id: '',
});

const areOptionsShown = ref<boolean>(false);

function openOptions(): void {
    areOptionsShown.value = true;
}

function closeOptions(): void {
    if (!areOptionsShown.value) return;

    areOptionsShown.value = false;
}

async function onCopy(): Promise<void> {
    try {
        await navigator.clipboard.writeText(props.id);
    } catch (error) {
        console.error(error);
    }

    closeOptions();
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
        border-radius: 3px;

        &:hover {
            background: var(--v-active-base);
        }
    }

    .options {
        position: absolute;
        top: 0;
        right: 45px;
        width: 140px;
        height: auto;
        background: var(--v-background-base);
        border-radius: var(--br-table);
        font-family: 'font_medium', sans-serif;
        border: 1px solid var(--v-border-base);
        font-size: 14px;
        color: var(--v-header-base);
        z-index: 999;

        &__item {
            box-sizing: border-box;
            padding: 16px;
            cursor: pointer;
            text-align: left;
            color: var(--v-header-base);

            &:hover {
                background: var(--v-active-base);
            }
        }
    }
</style>
