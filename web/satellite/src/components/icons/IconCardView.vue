// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="6.99902" y="6.99951" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
        <rect x="6.99902" y="13.0005" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
        <rect x="12.999" y="6.99951" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
        <rect x="12.999" y="13.0005" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    size?: number | string;
}>(), {
    size: 24,
});
</script>
