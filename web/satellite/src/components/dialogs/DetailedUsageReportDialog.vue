// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="dialog"
        activator="parent"
        width="auto"
        min-width="400px"
        transition="fade-transition"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            Get Detailed Usage Report
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="dialog = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-form class="pa-6">
                <p class="text-subtitle-2 mb-2">Select date range to generate your report:</p>
                <v-chip-group v-model="option" mandatory filter color="primary">
                    <v-chip :value="Options.Month">Past Month</v-chip>
                    <v-chip :value="Options.Year">Past Year</v-chip>
                    <v-chip :value="Options.Custom">Choose Dates</v-chip>
                </v-chip-group>
                <v-date-picker
                    v-if="option === Options.Custom"
                    v-model="customRange"
                    header="Choose Dates"
                    multiple
                    show-adjacent-months
                    border
                    elevation="0"
                    rounded="lg"
                />
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block @click="downloadReport">Download Report</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VDialog,
    VBtn,
    VRow,
    VCol,
    VSheet,
    VCard,
    VCardItem,
    VCardTitle,
    VCardActions,
    VDivider,
    VChipGroup,
    VChip,
    VForm,
    VDatePicker,
} from 'vuetify/components';

import { Download } from '@/utils/download';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';

enum Options {
    Month = 0,
    Year,
    Custom,
}

const projectsStore = useProjectsStore();
const notify = useNotify();

const props = withDefaults(defineProps<{
    projectID?: string
}>(), {
    projectID: '',
});

const dialog = ref<boolean>(false);
const option = ref<Options>(Options.Month);
const since = ref<Date>();
const before = ref<Date>();
const customRange = ref<Date[]>([]);

/**
 * Sets past month as active option.
 */
function setPastMonth(): void {
    const now = new Date();

    since.value = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, now.getUTCDate(), now.getUTCHours(), 0, 0, 0));
    before.value = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), 0, 0, 0));
    option.value = Options.Month;
}

/**
 * Sets past year as active option.
 */
function setPastYear(): void {
    const now = new Date();

    since.value = new Date(Date.UTC(now.getUTCFullYear() - 1, now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), 0, 0, 0));
    before.value = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), 0, 0, 0));
    option.value = Options.Year;
}

/**
 * Sets custom date range as active option.
 */
function setChooseDates(): void {
    since.value = undefined;
    before.value = undefined;
    option.value = Options.Custom;
}

function downloadReport(): void {
    if (!(since.value && before.value)) {
        notify.error('Please select date range', AnalyticsErrorEventSource.DETAILED_USAGE_REPORT_MODAL);
        return;
    }

    try {
        const link = projectsStore.getUsageReportLink(since.value, before.value, props.projectID);
        Download.fileByLink(link);
        notify.success('Usage report download started successfully.');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.DETAILED_USAGE_REPORT_MODAL);
    }
}

watch(customRange, () => {
    if (customRange.value.length > 2) {
        customRange.value = customRange.value.slice(0, 2);
        return;
    }

    if (customRange.value.length < 2) {
        since.value = undefined;
        before.value = undefined;
        return;
    }

    let [start, end] = customRange.value;
    if (start.getTime() > end.getTime()) {
        [start, end] = [end, start];
    }

    since.value = new Date(Date.UTC(start.getFullYear(), start.getMonth(), start.getDate(), start.getHours(), 0, 0, 0));
    before.value = new Date(Date.UTC(end.getFullYear(), end.getMonth(), end.getDate(), 23, 59, 59, 999));
});

watch(option, () => {
    switch (option.value) {
    case Options.Month:
        setPastMonth();
        break;
    case Options.Year:
        setPastYear();
        break;
    case Options.Custom:
        setChooseDates();
    }
}, { immediate: true });
</script>
