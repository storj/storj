// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <a
        class="share-button"
        :href="item.link"
        target="_blank"
        rel="noopener noreferrer"
        :aria-label="item.label"
        :style="style"
    >
        <component :is="item.image" />
        <span>{{ item.label }}</span>
    </a>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { ShareButtonConfig } from '@/types/browser';

const props = defineProps<{ item: ShareButtonConfig }>();

/**
 * Returns share button background color.
 */
const style = computed((): Record<string, string> => {
    return { 'background-color': props.item.color };
});
</script>

<style scoped lang="scss">
    .share-button {
        display: flex;
        align-items: center;
        text-decoration: none;
        color: #fff;
        margin-right: 1em;
        margin-bottom: 1em;
        border-radius: 5px;
        transition: 25ms ease-out;
        padding: 0.5em 0.75em;
        font-size: 14px;
        font-family: 'font_regular', sans-serif;

        svg {
            width: 12px;
            height: 12px;
            margin-right: 5px;
        }
    }
</style>
