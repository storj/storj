// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- All Projects Icon -->
    <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M5.33455 11.8182L2 10L5.33455 8.18182" :stroke="color" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M14.6655 8.18182L18.0001 10L14.6655 11.8182" :stroke="color" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M18 13.6364L10 18L2 13.6364L5.33455 11.8182L10 14.3636L14.6655 11.8182L18 13.6364Z" :stroke="color" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M2 6.36364L10 10.7273L18 6.36364L10 2L2 6.36364Z" :stroke="color" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    color?: string;
}>(), {
    color: 'currentColor',
});
</script>
