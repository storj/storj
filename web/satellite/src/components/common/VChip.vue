// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chip" :class="{selected: isSelected}" @click="emits('select')">
        <component :is="icon" v-if="icon" class="chip__icon" />

        <span class="chip__text"> {{ label }} </span>
    </div>
</template>

<script setup lang="ts">
import { Component } from 'vue';

const props = withDefaults(defineProps<{
    label: string,
    isSelected?: boolean,
    icon?: string,
}>(), {
    isSelected: false,
    icon: undefined,
    label: '',
});

const emits = defineEmits(['select']);

</script>

<style scoped lang="scss">
.chip {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 2px;
    padding: 0 8px;
    border-radius: 24px;
    color: var(--c-grey-6);
    background: var(--c-white);
    cursor: pointer;

    :deep(path) {
        fill: var(--c-grey-6);
    }

    :deep(rect) {
        fill: var(--c-grey-6);
    }

    &__text {
        font-family: 'font_regular', sans-serif;
        font-size: 12px;
        line-height: 20px;
        font-weight: 700;
        text-transform: uppercase;
    }

    &.selected {
        color: var(--c-black);
        background: var(--c-grey-2);

        :deep(path),
        :deep(rect) {
            fill: var(--c-black);
        }
    }
}
</style>
