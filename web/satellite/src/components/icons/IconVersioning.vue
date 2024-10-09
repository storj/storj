// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M2.25 9C2.25 10.335 2.64588 11.6401 3.38758 12.7501C4.12928 13.8601 5.18349 14.7253 6.41689 15.2362C7.65029 15.7471 9.00749 15.8808 10.3169 15.6203C11.6262 15.3599 12.829 14.717 13.773 13.773C14.717 12.829 15.3599 11.6262 15.6203 10.3169C15.8808 9.00749 15.7471 7.65029 15.2362 6.41689C14.7253 5.18349 13.8601 4.12928 12.7501 3.38758C11.6401 2.64588 10.335 2.25 9 2.25C7.11296 2.2571 5.30173 2.99342 3.945 4.305L2.25 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M2.25 3.21436V6.00006H5.14286" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M9 5.14282V8.99997L12.8571 8.99991" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    size?: number | string;
}>(), {
    size: 18,
});
</script>
