// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <h1 class="text-h5 font-weight-bold my-2">{{ title }}</h1>
</template>

<script setup lang="ts">
defineProps<{
    title: string;
}>();
</script>
