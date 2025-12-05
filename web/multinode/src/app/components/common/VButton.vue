// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div
        class="container"
        :class="containerClassName"
        :style="style"
        @click="onPress"
    >
        <svg v-if="withPlus" class="plus" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M10 4.1665V15.8332" stroke="white" stroke-width="1.66667" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M4.16797 10H15.8346" stroke="white" stroke-width="1.66667" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span class="label">{{ label }}</span>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(defineProps<{
    label?: string;
    width?: string;
    height?: string;
    isWhite?: boolean;
    isTransparent?: boolean;
    isDeletion?: boolean;
    isDisabled?: boolean;
    withPlus?: boolean;
    inactive?: boolean;
    onPress?: () => void;
}>(), {
    label: 'Default',
    width: 'inherit',
    height: '48px',
    isWhite: false,
    isTransparent: false,
    isDeletion: false,
    isDisabled: false,
    withPlus: false,
    inactive: false,
    onPress: () => {},
});

const style = computed(() => {
    return { width: props.width, height: props.height };
});

const containerClassName = computed(() => {
    let className = `${props.inactive ? 'inactive' : ''}`;

    switch (true) {
    case props.isDisabled:
        className = 'disabled';
        break;
    case props.isWhite:
        className = 'white_btn';
        break;
    case props.isTransparent:
        className = 'transparent';
        break;
    case props.isDeletion:
        className = 'red_btn';
    }

    return className;
});
</script>

<style lang="scss" scoped>
    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--v-primary-base);
        border-radius: var(--br-button);
        cursor: pointer;

        .label {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            color: var(--c-button-label);
            margin: 0;
        }

        &:hover {
            background-color: var(--v-blue2-base);

            &.white_btn {
                box-shadow: none !important;
                background-color: var(--v-active-base) !important;
                border-color: transparent;
            }

            &.red_btn {
                box-shadow: none !important;
                background-color: var(--c-button-red-hover);
            }
        }
    }

    .plus {
        margin-right: 10px;
    }

    .red_btn {
        background-color: var(--v-error-base);
    }

    .white_btn {
        background-color: transparent;
        border: 1px solid var(--v-border-base);

        .label {
            color: var(--v-text-base);
        }

        .plus {

            path {
                stroke: var(--c-title);
            }
        }
    }

    .disabled {
        background-color: var(--c-button-disabled);
        pointer-events: none !important;

        .label {
            color: #acb0bc !important;
        }
    }

    .inactive {
        opacity: 0.5;
        pointer-events: none !important;
    }
</style>
