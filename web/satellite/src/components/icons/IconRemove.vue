// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path :fill="color" d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM12.4475 10.873H7.552C7.09637 10.873 6.727 10.5036 6.727 10.048C6.727 9.60348 7.07857 9.24107 7.51882 9.22366L7.552 9.223H12.3926C12.8529 9.223 13.2302 9.58799 13.2456 10.048C13.2602 10.4887 12.9148 10.8579 12.4741 10.8726C12.4652 10.8729 12.4564 10.873 12.4475 10.873Z" />
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    color?: string;
    size?: number | string;
}>(), {
    color: 'currentColor',
    size: 20,
});
</script>
