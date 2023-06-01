// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <GalleryView v-if="isGalleryView" />
    <div v-if="activeModal">
        <component :is="activeModal" />
    </div>
</template>

<script setup lang="ts">
import { computed, Component } from 'vue';

import { useAppStore } from '@/store/modules/appStore';

import GalleryView from '@/components/browser/galleryView/GalleryView.vue';

const appStore = useAppStore();

/**
 * Indicates the current active modal.
 */
const activeModal = computed((): Component | null => {
    return appStore.state.activeModal;
});

/**
 * Indicates gallery view is visible.
 */
const isGalleryView = computed((): boolean => {
    return appStore.state.isGalleryView;
});
</script>
