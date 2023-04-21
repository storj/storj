// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="dropdown" v-click-outside="close" class="side-dropdown" :style="style">
        <slot />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

const props = withDefaults(defineProps<{
    yPosition: number;
    xPosition: number;
    close: () => void;
}>(), {
    yPosition: 0,
    xPosition: 0,
    close: () => {},
});

const dropdown = ref<HTMLDivElement>();
const dropdownMiddle = ref<number>(0);

/**
 * Returns top and left position of dropdown.
 */
const style = computed((): Record<string, string> => {
    return { top: `${props.yPosition - dropdownMiddle.value}px`, left: `${props.xPosition}px` };
});

/**
 * Mounted hook after initial render.
 * Calculates dropdowns Y middle point.
 */
onMounted((): void => {
    dropdownMiddle.value = (dropdown.value?.getBoundingClientRect().height || 0) / 2;
});
</script>

<style scoped lang="scss">
    .side-dropdown {
        position: absolute;
        background: #fff;
        border: 1px solid var(--c-grey-2);
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        border-radius: 8px;
        width: 390px;
        z-index: 1;
    }
</style>
