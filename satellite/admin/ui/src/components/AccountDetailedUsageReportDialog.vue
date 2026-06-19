// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="435px"
        width="auto"
        transition="fade-transition"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="FileSpreadsheet" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Get Detailed Usage Report
                    </v-card-title>
                    <v-card-subtitle>
                        For account: {{ userID }}
                    </v-card-subtitle>

                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-form class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <p class="text-subtitle-2 mb-2">Report Period</p>
                        <v-chip-group v-model="option" mandatory filter>
                            <v-chip :value="Options.Month" variant="outlined">Past Month</v-chip>
                            <v-chip :value="Options.ThreeMonths" variant="outlined">Past 3 Months</v-chip>
                            <v-chip :value="Options.Custom" variant="outlined">Custom Range</v-chip>
                        </v-chip-group>
                        <v-date-picker
                            v-if="option === Options.Custom"
                            v-model="customRange"
                            :allowed-dates="allowDate"
                            :min="minDate"
                            header="Choose Dates"
                            multiple="range"
                            show-adjacent-months
                            border
                            elevation="0"
                            rounded="lg"
                            class="w-100 mt-4"
                        />
                    </v-col>

                    <v-col cols="12">
                        <p class="text-subtitle-2 mb-2">Usage Details</p>
                        <v-chip-group v-model="projectSummary" mandatory filter>
                            <v-chip :value="true" variant="outlined">Project Summary</v-chip>
                            <v-chip :value="false" variant="outlined">Bucket Details</v-chip>
                        </v-chip-group>
                    </v-col>

                    <v-col cols="12">
                        <v-alert>
                            Your report will include:
                            <p v-if="includedDateString">Usage from {{ includedDateString }}.</p>
                            <p v-if="projectSummary">Summary of all project(s) usage for this account.</p>
                            <p v-else>Summary of all bucket(s) usage for this account.</p>
                        </v-alert>
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
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
import { computed, ref, watch } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardSubtitle,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol,
    VDatePicker,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSheet,
} from 'vuetify/components';
import { useDate } from 'vuetify/framework';
import { FileSpreadsheet, X } from 'lucide-vue-next';

import { UUID } from '@/types/common';
import { useBillingStore } from '@/store/billing';
import { useNotify } from '@/composables/useNotify';
import { Download } from '@/utils/download';

enum Options {
    Month = 0,
    ThreeMonths,
    Custom,
}

const props = defineProps<{
    userID: UUID;
}>();

const billingStore = useBillingStore();
const notify = useNotify();
const dateFns = useDate();

const model = defineModel<boolean>({ required: true });

const option = ref<Options>(Options.Month);
const projectSummary = ref(true);
const since = ref<Date>();
const before = ref<Date>();
const customRange = ref<Date[]>([]);

const minDate = computed<string>(() => {
    const today = new Date();
    today.setMonth(today.getMonth() - 3);
    return today.toISOString().split('T')[0];
});

const includedDateString = computed(() => {
    if (!since.value || !before.value) return '';

    const sinceStr = dateFns.format(since.value, 'monthAndYear');
    const beforeStr = dateFns.format(before.value, 'monthAndYear');

    if (sinceStr === beforeStr) {
        return sinceStr;
    }

    return `${sinceStr} - ${beforeStr}`;
});

function setPastMonth(): void {
    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth();
    since.value = new Date(Date.UTC(year, month - 1, 1, 0, 0, 0, 0));
    before.value = new Date(Date.UTC(year, month, 0, 23, 59, 59, 999));
    option.value = Options.Month;
}

function setPastThreeMonths(): void {
    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth();
    since.value = new Date(Date.UTC(year, month - 3, 1, 0, 0, 0, 0));
    before.value = new Date(Date.UTC(year, month, 0, 23, 59, 59, 999));
    option.value = Options.ThreeMonths;
}

function setChooseDates(): void {
    since.value = undefined;
    before.value = undefined;
    option.value = Options.Custom;
    customRange.value = [];
}

function allowDate(date: unknown): boolean {
    if (!date) return false;
    const d = new Date(date as string);
    if (isNaN(d.getTime())) return false;

    d.setHours(0, 0, 0, 0);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    return d <= today;
}

function downloadReport(): void {
    if (!(since.value && before.value)) {
        notify.error('Please select date range');
        return;
    }

    const link = billingStore.getUsageReportLink(props.userID, since.value, before.value, projectSummary.value);
    Download.fileByLink(link);
    notify.success('Usage report download started successfully.');
    model.value = false;
}

watch(customRange, (newRange) => {
    if (newRange.length < 2) {
        since.value = undefined;
        before.value = undefined;
        return;
    }

    let start = newRange[0];
    let end = newRange[newRange.length - 1];
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
    case Options.ThreeMonths:
        setPastThreeMonths();
        break;
    case Options.Custom:
        setChooseDates();
    }
}, { immediate: true });
</script>
