// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <PageTitleComponent
                    title="Daily Usage"
                    extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
                />
                <PageSubtitleComponent subtitle="Select date range to view daily usage statistics." />
            </v-col>
            <v-col cols="auto" class="pt-0 mt-2 mt-md-0 pt-md-7">
                <v-date-input
                    v-model="chartDateRange"
                    :min="minDatePickerDate"
                    :max="maxDatePickerDate"
                    label="Select Date Range"
                    min-width="265px"
                    multiple="range"
                    prepend-icon=""
                    density="comfortable"
                    variant="outlined"
                    :loading="isLoading"
                    class="bg-surface"
                    show-adjacent-months
                    hide-details
                >
                    <v-icon class="mr-2" size="20" :icon="Calendar" />
                </v-date-input>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" md="6">
                <v-card ref="chartContainer" class="pa-1 pb-3">
                    <template #title>
                        <v-card-title class="d-flex align-center">
                            <v-icon :icon="Cloud" size="small" color="primary" class="mr-2" />
                            Storage
                        </v-card-title>
                    </template>
                    <v-card-item class="pt-1">
                        <v-card class="dot-background" rounded="md">
                            <StorageChart
                                :width="chartWidth"
                                :height="240"
                                :data="storageUsage"
                                :since="chartsSinceDate"
                                :before="chartsBeforeDate"
                            />
                        </v-card>
                    </v-card-item>
                </v-card>
            </v-col>
            <v-col cols="12" md="6">
                <v-card class="pa-1 pb-3">
                    <template #title>
                        <v-card-title class="d-flex align-center justify-space-between">
                            <v-row class="ma-0 align-center">
                                <v-icon :icon="CloudDownload" size="small" color="primary" class="mr-2" />
                                Download
                                <v-tooltip width="240" location="bottom">
                                    <template #activator="{ props }">
                                        <v-icon v-bind="props" size="12" :icon="Info" class="ml-2 text-medium-emphasis" />
                                    </template>
                                    <template #default>
                                        <p>
                                            Download bandwidth appears here after downloads complete or cancel within 48 hours.
                                        </p>
                                    </template>
                                </v-tooltip>
                            </v-row>
                        </v-card-title>
                    </template>
                    <v-card-item class="pt-1">
                        <v-card class="dot-background" rounded="md">
                            <BandwidthChart
                                :width="chartWidth"
                                :height="240"
                                :data="settledBandwidthUsage"
                                :since="chartsSinceDate"
                                :before="chartsBeforeDate"
                            />
                        </v-card>
                    </v-card-item>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch , ComponentPublicInstance } from 'vue';
import {
    VCard,
    VCardTitle,
    VCardItem,
    VCol,
    VContainer,
    VRow,
    VIcon,
    VTooltip,
} from 'vuetify/components';
import { VDateInput } from 'vuetify/labs/components';
import { Info, Cloud, CloudDownload, Calendar } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { DataStamp } from '@/types/projects';
import { ChartUtils } from '@/utils/chart';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import BandwidthChart from '@/components/BandwidthChart.vue';
import StorageChart from '@/components/StorageChart.vue';

const projectsStore = useProjectsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const chartWidth = ref<number>(0);
const chartContainer = ref<ComponentPublicInstance>();
const datePickerModel = ref<Date[]>([]);

/**
 * Returns charts since date from store.
 */
const chartsSinceDate = computed((): Date => {
    return projectsStore.state.chartDataSince;
});

/**
 * Returns charts before date from store.
 */
const chartsBeforeDate = computed((): Date => {
    return projectsStore.state.chartDataBefore;
});

/**
 * Return a new 7 days range if datePickerModel is empty.
 */
const chartDateRange = computed<Date[]>({
    get: () => {
        const dates: Date[] = [...datePickerModel.value];
        if (!dates.length) {
            for (let i = 6; i >= 0; i--) {
                const d = new Date();
                d.setDate(d.getDate() - i);
                dates.push(d);
            }
        }
        return dates;
    },
    set: newValue => {
        const newRange = [...newValue];
        if (newRange.length === 0) {
            return;
        }
        if (newRange.length < 2) {
            const d = new Date();
            d.setDate(newRange[0].getDate() + 1);
            newRange.push(d);
        }
        datePickerModel.value = newRange;
    },
});

/**
 * Returns storage chart data from store.
 */
const storageUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.storageChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns allocated bandwidth chart data from store.
 */
const settledBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.settledBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

const minDatePickerDate = computed<string>(() => {
    const d = new Date();
    d.setFullYear(d.getFullYear() - 1);
    return d.toISOString().split('T')[0];
});

const maxDatePickerDate = computed<string>(() => {
    const d = new Date();
    return d.toISOString().split('T')[0];
});

/**
 * Used container size recalculation for charts resizing.
 */
function recalculateChartWidth(): void {
    chartWidth.value = chartContainer.value?.$el.getBoundingClientRect().width - 16 || 0;
}

/**
 * Lifecycle hook after initial render.
 * Fetches project limits.
 */
onMounted(() => {
    window.addEventListener('resize', recalculateChartWidth);
    recalculateChartWidth();

    withLoading(async () => {
        try {
            await projectsStore.getDailyProjectData({ since: chartDateRange.value[0], before: chartDateRange.value[chartDateRange.value.length - 1] });
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
        }
    });
});

/**
 * Lifecycle hook before component destruction.
 * Removes event listener on window resizing.
 */
onBeforeUnmount((): void => {
    window.removeEventListener('resize', recalculateChartWidth);
});

watch(datePickerModel, (newRange) => {
    if (newRange.length < 2) return;

    withLoading(async () => {
        let startDate = newRange[0];
        let endDate = newRange[newRange.length - 1];
        if (startDate.getTime() > endDate.getTime()) {
            [startDate, endDate] = [endDate, startDate];
        }

        const since = new Date(startDate);
        const before = new Date(endDate);
        before.setHours(23, 59, 59, 999);

        try {
            await projectsStore.getDailyProjectData({ since, before });
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
        }
    });
});
</script>

<style scoped lang="scss">
:deep(.v-field__input) {
    cursor: pointer;

    input {
        cursor: pointer;
    }
}

.dot-background {
    background-image: radial-gradient(circle, rgb(var(--v-theme-on-surface),0.04) 1px, transparent 1px);
    background-size: 12px 12px;
    background-color: rgb(var(--v-theme-surface));;
}
</style>
