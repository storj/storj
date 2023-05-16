// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="activeModal">
        <component :is="activeModal" />
    </div>
</template>

<script setup lang="ts">
import { Component, computed } from 'vue';

import { useAppStore } from '@/store/modules/appStore';

const appStore = useAppStore();

/**
 * Indicates the current active modal.
 */
const activeModal = computed((): Component | null => {
    return appStore.state.activeModal;
});
</script>
