// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" :subtitle="cardSubtitle" variant="flat" :border="true" rounded="xlg" height="100%">
        <template v-if="tooltip" #title>
            <div class="d-flex align-center">
                <span class="text-truncate">{{ title }}</span>
                <v-tooltip :text="tooltip" max-width="300">
                    <template #activator="{ props: activatorProps }">
                        <v-icon v-bind="activatorProps" :icon="Info" size="16" class="ml-1 flex-shrink-0 text-medium-emphasis" />
                    </template>
                </v-tooltip>
            </div>
        </template>
        <template v-if="featureFlags.account.updateStatus && !props.updateDisabled" #append>
            <v-btn :icon="Settings2" variant="outlined" size="small" density="comfortable" color="default" @click="emit('updateLimits')" />
        </template>
        <v-card-text>
            <template v-if="usedLabel">
                <p class="text-body-2 text-medium-emphasis">{{ usedLabel }}</p>
                <v-chip variant="tonal" class="mt-1 font-weight-bold">{{ used }}</v-chip>
            </template>
            <v-chip v-else variant="tonal" :color="color" class="font-weight-bold">{{ used }} / {{ limit }}</v-chip>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VIcon, VTooltip } from 'vuetify/components';
import { computed } from 'vue';
import { Info, Settings2 } from 'lucide-vue-next';

import { useAppStore } from '@/store/app';
import { Size } from '@/utils/bytesSize';

const appStore = useAppStore();

const featureFlags = computed(() => appStore.state.settings.admin.features);

const props = defineProps<{
    title: string;
    used: Size | number;
    subtitle?: string;
    limit: Size | number;
    updateDisabled?: boolean;
    tooltip?: string;
    // When set, the limit is shown as a default limit in the subtitle and the
    // usage is shown under this label, without comparing the two.
    usedLabel?: string;
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

const cardSubtitle = computed((): string | undefined => {
    return props.usedLabel ? `${limit.value} default limit` : props.subtitle;
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