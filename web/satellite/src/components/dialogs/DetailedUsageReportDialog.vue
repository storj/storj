// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="dialog"
        activator="parent"
        max-width="420px"
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
                    <v-card-subtitle v-if="project">
                        For project {{ project.name }}
                    </v-card-subtitle>

                    <template #append>
                        <v-btn
                            :icon="X"
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
                <v-row>
                    <v-col cols="12">
                        <p class="text-subtitle-2 mb-2">Report Period</p>
                        <v-chip-group v-model="option" mandatory filter>
                            <v-chip :value="Options.Month" variant="outlined">Past Month</v-chip>
                            <v-chip :value="Options.Year" variant="outlined">Past Year</v-chip>
                            <v-chip :value="Options.Custom" variant="outlined">Custom Range</v-chip>
                        </v-chip-group>
                        <v-date-picker
                            v-if="option === Options.Custom"
                            v-model="customRange"
                            :allowed-dates="allowDate"
                            header="Choose Dates"
                            multiple="range"
                            show-adjacent-months
                            border
                            elevation="0"
                            rounded="lg"
                            class="w-100 mt-4"
                        />
                    </v-col>

                    <template v-if="newUsageReportEnabled">
                        <v-col cols="12">
                            <p class="text-subtitle-2 mb-2">Usage Details</p>
                            <v-chip-group v-model="projectSummary" mandatory filter>
                                <v-chip :value="true" variant="outlined">Project Summary</v-chip>
                                <v-chip :value="false" variant="outlined">Bucket Details</v-chip>
                            </v-chip-group>
                        </v-col>

                        <v-col cols="12">
                            <p class="text-subtitle-2 mb-2">Cost Information</p>
                            <v-chip-group v-model="includeCost" mandatory filter>
                                <v-chip :value="true" variant="outlined">Include Cost</v-chip>
                                <v-chip :value="false" variant="outlined">Usage Only</v-chip>
                            </v-chip-group>
                        </v-col>

                        <v-col cols="12">
                            <v-alert>
                                Your report will include:
                                <p v-if="includedDateString">Usage from {{ includedDateString }}.</p>
                                <p v-if="projectSummary">
                                    <span v-if="!project">Summary of your project(s) usage.</span>
                                    <span v-else>Summary of your usage in the project <b>{{ project.name }}</b>.</span>
                                </p>
                                <p v-else>
                                    <span v-if="!project">Summary of your bucket(s) usage.</span>
                                    <span v-else>Summary of your bucket(s) usage in the project <b>{{ project.name }}</b>.</span>
                                </p>
                                <p v-if="includeCost">Detailed cost breakdown.</p>
                            </v-alert>
                        </v-col>
                    </template>
                </v-row>
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

import { Download } from '@/utils/download';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';
import { Project } from '@/types/projects';

enum Options {
    Month = 0,
    Year,
    Custom,
}

const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const dateFns = useDate();

const props = defineProps<{
    project?: Project;
}>();

const dialog = ref<boolean>(false);
const option = ref<Options>(Options.Month);
const projectSummary = ref(true);
const includeCost = ref(true);
const since = ref<Date>();
const before = ref<Date>();
const customRange = ref<Date[]>([]);

const newUsageReportEnabled = computed(() => configStore.state.config.newDetailedUsageReportEnabled);

const includedDateString = computed(() => {
    if (!since.value || !before.value) return '';

    const sinceStr = dateFns.format(since.value, 'monthAndYear');
    const beforeStr = dateFns.format(before.value, 'monthAndYear');

    if (sinceStr === beforeStr) {
        return sinceStr;
    }

    return `${sinceStr} - ${beforeStr}`;
});

/**
 * Sets past month as active option.
 */
function setPastMonth(): void {
    since.value = (dateFns.getPreviousMonth(new Date()) as Date);
    before.value = dateFns.endOfMonth(since.value) as Date;
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
        notify.error('Please select date range', AnalyticsErrorEventSource.DETAILED_USAGE_REPORT_MODAL);
        return;
    }

    try {
        const link = projectsStore.getUsageReportLink(since.value, before.value, includeCost.value, projectSummary.value, props.project?.id);
        Download.fileByLink(link);
        notify.success('Usage report download started successfully.');
        dialog.value = false;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.DETAILED_USAGE_REPORT_MODAL);
    }
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
    case Options.Year:
        setPastYear();
        break;
    case Options.Custom:
        setChooseDates();
    }
}, { immediate: true });
</script>
