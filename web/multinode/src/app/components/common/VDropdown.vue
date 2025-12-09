// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        v-if="options.length"
        class="dropdown"
        :class="{ active: areOptionsShown }"
        @click.stop="toggleOptions"
    >
        <span v-if="selectedOption" class="label">{{ selectedOption.label }}</span>
        <svg width="8" height="4" viewBox="0 0 8 4" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M3.33657 3.73107C3.70296 4.09114 4.29941 4.08814 4.66237 3.73107L7.79796 0.650836C8.16435 0.291517 8.01864 0 7.47247 0L0.526407 0C-0.0197628 0 -0.16292 0.294525 0.200917 0.650836L3.33657 3.73107Z" fill="currentColor" />
        </svg>
        <div v-if="areOptionsShown" v-click-outside="closeOptions" class="dropdown__selection">
            <div class="dropdown__selection__overflow-container">
                <div v-for="option in options" :key="option.label" class="dropdown__selection__option" @click="onOptionClick(option)">
                    <span class="label">{{ option.label }}</span>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';

import { Option } from '@/app/types/common';

const props = withDefaults(defineProps<{
    options?: Option[];
    preselectedOption?: Option | null;
}>(), {
    options: () => [],
    preselectedOption: null,
});

const areOptionsShown = ref<boolean>(false);
const selectedOption = ref<Option>();

function toggleOptions(): void {
    areOptionsShown.value = !areOptionsShown.value;
}

function closeOptions(): void {
    if (!areOptionsShown.value) { return; }

    areOptionsShown.value = false;
}

async function onOptionClick(option: Option): Promise<void> {
    selectedOption.value = option;
    await option.onClick();
    closeOptions();
}

onBeforeMount(() => {
    selectedOption.value = props.preselectedOption || props.options[0];
});
</script>

<style lang="scss">
    .dropdown {
        position: relative;
        box-sizing: border-box;
        width: 100%;
        max-width: 300px;
        height: 40px;
        background: transparent;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 16px;
        border: 1px solid var(--v-border-base);
        border-radius: 6px;
        font-size: 16px;
        color: var(--v-text-base);
        cursor: pointer;
        font-family: 'font_medium', sans-serif;
        z-index: 998;

        &:hover {
            border-color: var(--c-gray);
        }

        &.active {
            border-color: var(--c-primary);
        }

        &__selection {
            position: absolute;
            top: 52px;
            left: 0;
            width: 100%;
            border: 1px solid var(--v-border-base);
            border-radius: 6px;
            overflow: hidden;
            background: var(--v-background-base);
            z-index: 999;

            &__overflow-container {
                overflow: auto;
                overflow-x: hidden;
                max-height: 160px;
            }

            &__option {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 0 16px;
                height: 40px;
                width: 100% !important;
                cursor: pointer;
                border-bottom: 1px solid var(--v-border-base);
                box-sizing: border-box;

                &:hover {
                    background: var(--v-active-base);
                }
            }
        }
    }

    .label {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        margin-right: 5px;
    }

    ::-webkit-scrollbar {
        width: 3px;
    }

    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px transparent;
    }

    ::-webkit-scrollbar-thumb {
        background: var(--c-gray--light);
        border-radius: 6px;
        height: 5px;
    }
</style>
