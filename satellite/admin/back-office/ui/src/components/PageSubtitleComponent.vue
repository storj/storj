// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="text-medium-emphasis">
        {{ subtitle }}
    </p>
</template>

<script setup lang="ts">
const props = defineProps<{
    subtitle: string;
}>();
</script>
