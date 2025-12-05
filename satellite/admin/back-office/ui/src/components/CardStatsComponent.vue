// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" :subtitle="subtitle" variant="flat" :border="true" rounded="xlg">
        <template v-if="featureFlags.account.updateStatus && !props.updateDisabled" #append>
            <v-btn :icon="Settings2" variant="outlined" size="small" density="comfortable" color="default" @click="emit('updateLimits')" />
        </template>
        <v-card-text>
            <v-chip variant="tonal" :color="color" class="font-weight-bold">{{ used }} / {{ limit }}</v-chip>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip } from 'vuetify/components';
import { computed } from 'vue';
import { Settings2 } from 'lucide-vue-next';

import { useAppStore } from '@/store/app';
import { Size } from '@/utils/bytesSize';

const appStore = useAppStore();

const featureFlags = computed(() => appStore.state.settings.admin.features);

const props = defineProps<{
    title: string;
    used: Size | number;
    subtitle: string;
    limit: Size | number;
    updateDisabled?: boolean;
}>();

const emit = defineEmits<{
    (e: 'updateLimits'): void;
}>();

const used = computed((): string => {
    if (typeof props.used === 'number') {
        return props.used.toString();
    }
    return props.used.formattedBytes + ' ' + props.used.label;
});

const limit = computed((): string => {
    if (typeof props.limit === 'number') {
        return props.limit.toString();
    }
    return props.limit.formattedBytes + ' ' + props.limit.label;
});

const color = computed((): string => {
    const limit = typeof props.limit === 'number' ? props.limit : props.limit.bytes;
    const used = typeof props.used === 'number' ? props.used : props.used.bytes;
    if (limit <= used) {
        return 'error';
    }

    const p = used/limit * 100;
    if (p < 60) {
        return 'success';
    } else if (p < 80) {
        return 'warning';
    } else {
        return 'error';
    }
});
</script>