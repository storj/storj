// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="text-medium-emphasis">{{ subtitle }} <a :href="link" target="_blank" class="link">Learn More</a></p>
</template>

<script setup lang="ts">
const props = defineProps<{
    subtitle: string;
    link: string;
}>();
</script>
