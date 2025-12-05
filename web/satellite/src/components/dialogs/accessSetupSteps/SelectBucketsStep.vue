// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col>
                <p>Choose buckets you want to be accessible.</p>
                <v-chip-group
                    v-model="option"
                    filter
                    mandatory
                    selected-class="font-weight-bold"
                    variant="outlined"
                    class="mt-2 mb-3"
                    @update:model-value="onChangeOption"
                >
                    <v-chip
                        filter
                        :value="BucketsOption.All"
                    >
                        All Buckets
                    </v-chip>

                    <v-chip
                        filter
                        :value="BucketsOption.Select"
                    >
                        Select Buckets
                    </v-chip>
                </v-chip-group>

                <v-alert v-if="option === BucketsOption.All" variant="tonal">
                    <p class="text-subtitle-2 font-weight-bold">All Buckets</p>
                    <p class="text-subtitle-2">The application can access all of the current and future buckets you create in this project.</p>
                </v-alert>

                <v-alert v-else variant="tonal" color="primary">
                    <p class="text-subtitle-2 font-weight-bold">Select Buckets</p>
                    <p class="text-subtitle-2">The application can access the selected buckets in this project.</p>
                    <v-autocomplete
                        v-model="buckets"
                        v-model:search="bucketSearch"
                        :items="allBucketNames"
                        class="mt-4"
                        variant="outlined"
                        label="Buckets"
                        placeholder="Select buckets"
                        no-data-text="No buckets found."
                        multiple
                        chips
                        closable-chips
                        hide-details
                        :custom-filter="bucketFilter"
                    >
                        <template #item="{ props: slotProps }">
                            <v-list-item v-bind="slotProps" density="compact">
                                <template #prepend="{ isSelected }">
                                    <v-checkbox-btn :model-value="isSelected" />
                                </template>
                            </v-list-item>
                        </template>
                    </v-autocomplete>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VAlert,
    VAutocomplete,
    VCheckboxBtn,
    VChip,
    VChipGroup,
    VCol,
    VForm,
    VListItem,
    VRow,
} from 'vuetify/components';

import { BucketsOption } from '@/types/setupAccess';
import { useBucketsStore } from '@/store/modules/bucketsStore';

const bucketsStore = useBucketsStore();

const emit = defineEmits<{
    'bucketsChanged': [buckets: string[]];
}>();

const option = ref<BucketsOption>(BucketsOption.All);
const buckets = ref<string[]>([]);
const bucketSearch = ref<string>('');

/**
 * Returns all bucket names from the store.
 */
const allBucketNames = computed<string[]>(() => bucketsStore.state.allBucketNames);

/**
 * Returns whether the bucket name satisfies the query.
 */
function bucketFilter(bucketName: string, query: string): boolean {
    query = query.trim();
    if (!query) return true;

    let lastIdx = 0;
    for (const part of query.split(' ')) {
        const idx = bucketName.indexOf(part, lastIdx);
        if (idx === -1) return false;
        lastIdx = idx + part.length;
    }
    return true;
}

/**
 * Clears selected buckets.
 */
function onChangeOption(): void {
    buckets.value = [];
}

watch(buckets, value => {
    emit('bucketsChanged', value.slice());
}, { deep: true });
</script>
