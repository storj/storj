// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="text-medium-emphasis">{{ subtitle }} <a v-if="link" :href="link" target="_blank" rel="noopener noreferrer" class="link">Learn More</a></p>
</template>

<script setup lang="ts">
const props = defineProps<{
    subtitle: string;
    link?: string;
}>();
</script>
