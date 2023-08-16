// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <a
        class="share-button"
        :href="config.getLink(link)"
        target="_blank"
        rel="noopener noreferrer"
        :aria-label="props.option"
        :style="style"
    >
        <component :is="config.icon" width="12" height="12" />
        <span>{{ props.option }}</span>
    </a>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { ShareOptions, SHARE_BUTTON_CONFIGS, ShareButtonConfig } from '@/types/browser';

const props = defineProps<{
    option: ShareOptions;
    link: string;
}>();

/**
 * Returns share button background color.
 */
const style = computed((): Record<string, string> => {
    return { 'background-color': config.value.color };
});

/**
 * Returns the configuration for this button's share option.
 */
const config = computed((): ShareButtonConfig => {
    return SHARE_BUTTON_CONFIGS[props.option];
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
            margin-right: 5px;
        }
    }
</style>
