// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="button-icon" :class="{ active: isActive }" @click="onPress">
        <component :is="icon" />
    </div>
</template>

<script setup lang="ts">
import { Component } from 'vue';

const props = withDefaults(defineProps<{
    isActive?: boolean
    icon: string
    onPress: () => void
}>(), {
    isActive: false,
});
</script>

<style scoped lang="scss">
.button-icon {
    width: 48px;
    height: 48px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
}

.active {
    background: rgb(255 255 255 / 10%);
}
</style>
