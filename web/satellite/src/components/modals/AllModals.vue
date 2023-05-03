// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="activeModal">
        <component :is="activeModal" />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useAppStore } from '@/store/modules/appStore';

const appStore = useAppStore();

/**
 * Indicates the current active modal.
 */
const activeModal = computed((): unknown | null => {
    // modal could be of VueConstructor type or Object (for composition api components).
    return appStore.state.activeModal;
});
</script>
