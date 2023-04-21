// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="truncation-container">
        <div class="first-part-text">{{ textFirstPart }}</div>
        <div class="last-part-text">{{ textLastPart }}</div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
    text: string,
}>();

const textFirstPart = computed((): string => {
    return props.text.substring(0, props.text.length - 5);
});

const textLastPart = computed((): string => {
    return props.text.substring(props.text.length - 5);
});
</script>

<style scoped lang="scss">
.truncation-container {
    display: inline-flex;
    max-width: 30ch;
    min-width: 0;

    &:hover {
        max-width: unset;
        min-width: auto;
    }
}

.first-part-text {
    flex-grow: 1;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
}

.last-part-text {
    flex-shrink: 0;
}
</style>
