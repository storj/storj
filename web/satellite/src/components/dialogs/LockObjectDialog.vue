// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Lock" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Lock
                    </v-card-title>
                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="my-2">
                        Locking this version will prevent it from being deleted or overwritten for the specified period of time.
                    </p>

                    <p class="mt-4 mb-2 font-weight-bold text-body-2">
                        Name:
                    </p>

                    <v-chip
                        variant="tonal"
                        filter
                        value="filename"
                        color="default"
                        class="mb-2 font-weight-bold"
                    >
                        {{ file?.Key }}
                    </v-chip>

                    <p class="my-2 font-weight-bold text-body-2">
                        Version:
                    </p>

                    <v-chip
                        variant="tonal"
                        filter
                        color="default"
                        class="mb-4 font-weight-bold"
                    >
                        {{ file?.VersionId }}
                    </v-chip>

                    <p class="mb-2 font-weight-bold text-body-2">
                        Select the lock retention period:
                    </p>

                    <v-chip-group
                        v-model="selectedRange"
                        class="mb-4"
                        mandatory
                        column
                        filter
                    >
                        <v-chip v-for="range in ranges" :key="range.label" :value="range">
                            {{ range.label }}
                        </v-chip>
                    </v-chip-group>

                    <v-date-picker
                        v-if="selectedRange?.label == customRangeLabel.label"
                        v-model="customUntilDate"
                        width="100%"
                        header="Choose Date"
                        show-adjacent-months
                        border
                        elevation="0"
                        rounded="lg"
                    />
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            :disabled="isLoading"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="!selectedRange?.date && !customUntilDate"
                            :loading="isLoading"
                            block
                            @click="lockObject"
                        >
                            Set Lock
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { defineModel, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol, VDatePicker,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { Lock } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Time } from '@/utils/time';

interface LockUntilRange {
    label: string,
    date?: Date,
}

const customRangeLabel = { label: 'Choose a custom date' };
const ranges: LockUntilRange[] = [
    { label: '1 Day', date: dateAfterDays(1) },
    { label: '2 Days', date: dateAfterDays(2) },
    { label: '1 Week', date: dateAfterDays(7) },
    { label: '2 Weeks', date: dateAfterDays(14) },
    { label: '1 Month', date: dateAfterDays(30) },
    { label: '3 Months', date: dateAfterDays(90) },
    { label: '6 Months', date: dateAfterDays(180) },
    { label: '1 Year', date: dateAfterDays(365) },
    { label: '2 Years', date: dateAfterDays(730) },
    { label: '3 Years', date: dateAfterDays(1095) },
    { label: '5 Years', date: dateAfterDays(1825) },
    { label: '7 Years', date: dateAfterDays(2555) },
    { label: '10 Years', date: dateAfterDays(3650) },
    customRangeLabel,
];

const obStore = useObjectBrowserStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const model = defineModel<boolean>({ required: false });
const props = defineProps<{
    file: BrowserObject | null
}>();

const emit = defineEmits<{
    'contentRemoved': [],
    'fileLocked': [],
}>();

const innerContent = ref<VCard | null>(null);

const selectedRange = ref<LockUntilRange>();

const customUntilDate = ref<Date>();

function dateAfterDays(days: number): Date {
    const laterDate = new Date();
    laterDate.setDate(new Date().getDate() + days);
    return laterDate;
}

function lockObject() {
    withLoading(async () => {
        if (!props.file) {
            return;
        }
        let date: Date | undefined;
        if (selectedRange.value?.label === customRangeLabel.label) {
            date = customUntilDate.value;
        } else {
            date = selectedRange.value?.date;
        }
        if (!date) {
            notify.warning('Select a date');
            return;
        }
        try {
            await obStore.lockObject(props.file, date);
            notify.success(`Object locked until ${Time.formattedDate(date)}`);
            emit('fileLocked');
            model.value = false;
        } catch (e) {
            notify.notifyError(e, AnalyticsErrorEventSource.LOCK_OBJECT_DIALOG);
            return;
        }
    });
}

watch(selectedRange, (_) => {
    customUntilDate.value = undefined;
});

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>
