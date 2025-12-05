// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path fill-rule="evenodd" clip-rule="evenodd" d="M9 8C9 8.55228 8.55228 9 8 9V9C7.44772 9 7 8.55228 7 8V8C7 7.44772 7.44772 7 8 7V7C8.55228 7 9 7.44772 9 8V8Z" fill="currentColor" />
        <path fill-rule="evenodd" clip-rule="evenodd" d="M9 12C9 12.5523 8.55228 13 8 13V13C7.44772 13 7 12.5523 7 12V12C7 11.4477 7.44772 11 8 11V11C8.55228 11 9 11.4477 9 12V12Z" fill="currentColor" />
        <path fill-rule="evenodd" clip-rule="evenodd" d="M9 16C9 16.5523 8.55228 17 8 17V17C7.44772 17 7 16.5523 7 16V16C7 15.4477 7.44772 15 8 15V15C8.55228 15 9 15.4477 9 16V16Z" fill="currentColor" />
        <path fill-rule="evenodd" clip-rule="evenodd" d="M18 8C18 8.55228 17.5523 9 17 9H11C10.4477 9 10 8.55228 10 8V8C10 7.44772 10.4477 7 11 7H17C17.5523 7 18 7.44772 18 8V8Z" fill="currentColor" />
        <path fill-rule="evenodd" clip-rule="evenodd" d="M18 12C18 12.5523 17.5523 13 17 13H11C10.4477 13 10 12.5523 10 12V12C10 11.4477 10.4477 11 11 11H17C17.5523 11 18 11.4477 18 12V12Z" fill="currentColor" />
        <path fill-rule="evenodd" clip-rule="evenodd" d="M18 16C18 16.5523 17.5523 17 17 17H11C10.4477 17 10 16.5523 10 16V16C10 15.4477 10.4477 15 11 15H17C17.5523 15 18 15.4477 18 16V16Z" fill="currentColor" />
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    size?: number | string;
}>(), {
    size: 24,
});
</script>
