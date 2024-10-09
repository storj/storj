// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M3 1.7954C3 1.20488 3.62145 0.820799 4.14963 1.08489L16.8596 7.43988C17.4451 7.73262 17.4451 8.56815 16.8596 8.8609L4.58875 14.9963V18.2056C4.58875 18.6443 4.23309 19 3.79437 19C3.36635 19 3.0174 18.6615 3.00063 18.2376L3 18.2056V1.7954ZM4.58875 3.08069V13.2199L14.728 8.15039L4.58875 3.08069Z" fill="currentColor" />    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    size?: number | string;
}>(), {
    size: 16,
});
</script>

