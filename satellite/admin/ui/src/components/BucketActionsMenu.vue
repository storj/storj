// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item v-if="hasUpdatePerm" density="comfortable" link rounded="lg" @click="emit('update', bucket)">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Bucket
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VList, VListItem, VListItemTitle, VMenu } from 'vuetify/components';
import { computed } from 'vue';

import { BucketFlags, BucketInfo } from '@/api/client.gen';
import { useAppStore } from '@/store/app';

defineProps<{
    bucket: BucketInfo;
}>();

const emit = defineEmits<{
    (e: 'update', bucket: BucketInfo): void;
}>();

const featureFlags = computed(() => useAppStore().state.settings.admin.features.bucket as BucketFlags);

const hasUpdatePerm = computed(() => {
    return featureFlags.value.updatePlacement ||
        featureFlags.value.updateValueAttribution;
});
</script>
