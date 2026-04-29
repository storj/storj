// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col cols="12" md="auto">
                <PageTitleComponent
                    title="Project Usage"
                    extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
                />
            </v-col>
        </v-row>
        <v-row align="center" justify="center" class="mt-2">
            <v-col cols="6" md="6" :lg="statsRowLgColSize">
                <CardStatsComponent
                    title="Objects"
                    subtitle="Project total"
                    :data="limits.objectCount.toLocaleString()"
                    :to="ROUTES.Buckets.path"
                    color="info"
                    extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
                />
            </v-col>
            <v-col v-if="!emissionImpactViewEnabled && !newPricingEnabled && segmentsUIEnabled" cols="6" md="6" :lg="statsRowLgColSize">
                <CardStatsComponent title="Segments" color="info" subtitle="All object pieces" :data="limits.segmentCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col cols="6" md="6" :lg="statsRowLgColSize">
                <CardStatsComponent title="Buckets" color="info" subtitle="In this project" :data="bucketsCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col cols="6" md="6" :lg="statsRowLgColSize">
                <CardStatsComponent title="Access Keys" color="info" subtitle="Total keys" :data="accessGrantsCount.toLocaleString()" :to="ROUTES.Access.path" />
            </v-col>
            <v-col cols="6" md="6" :lg="statsRowLgColSize">
                <CardStatsComponent title="Team" color="info" subtitle="Project members" :data="teamSize.toLocaleString()" :to="ROUTES.Team.path" />
            </v-col>
            <template v-if="emissionImpactViewEnabled">
                <v-col cols="12" sm="6" md="6" :lg="statsRowLgColSize">
                    <emissions-dialog />
                    <v-tooltip
                        activator="parent"
                        location="top"
                        offset="-20"
                        opacity="80"
                    >
                        Click to learn more
                    </v-tooltip>
                    <CardStatsComponent title="CO₂ Estimated" subtitle="For this project" color="info" :data="co2Estimated" link />
                </v-col>
                <v-col cols="12" sm="6" md="6" :lg="statsRowLgColSize">
                    <emissions-dialog />
                    <v-tooltip
                        activator="parent"
                        location="top"
                        offset="-20"
                        opacity="80"
                    >
                        Click to learn more
                    </v-tooltip>
                    <CardStatsComponent title="CO₂ Avoided" :subtitle="avoidedSubtitle" :data="co2Saved" color="success" link />
                </v-col>
            </template>
            <v-col v-if="billingEnabled && !emissionImpactViewEnabled && !isMemberAccount" cols="6" md="6" :lg="statsRowLgColSize">
                <CardStatsComponent title="Billing" :subtitle="`${paidTierString} account`" :data="paidTierString" :to="ROUTES.Account.with(ROUTES.Billing).path" />
            </v-col>
        </v-row>
        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <PageTitleComponent title="Daily Usage" />
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
                            <v-row no-gutters class="ma-0 align-center">
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
import { type ComponentPublicInstance, computed, onBeforeUnmount, onMounted, ref, watch   } from 'vue';
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
import type { DataStamp, Emission, Project, ProjectLimits } from '@/types/projects';
import { ChartUtils } from '@/utils/chart';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import BandwidthChart from '@/components/BandwidthChart.vue';
import StorageChart from '@/components/StorageChart.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';
import EmissionsDialog from '@/components/dialogs/EmissionsDialog.vue';

type ValueUnit = {
    value: number
    unit: string
};

const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const agStore = useAccessGrantsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const chartWidth = ref<number>(0);
const chartContainer = ref<ComponentPublicInstance>();
const datePickerModel = ref<Date[]>([]);

const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

/**
 * Returns current team size from store.
 */
const teamSize = computed<number>(() => projectsStore.state.selectedProjectConfig.membersCount);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

/**
 * Whether this project has new pricing.
 */
const newPricingEnabled = computed<boolean>(() => {
    if (!billingEnabled.value) return false;
    return configStore.getProjectHasNewPricing(selectedProject.value.createdAt);
});

/**
 * Calculates stats row column size based on enabled cards.
 */
const statsRowLgColSize = computed(() => {
    let cards = 4;
    if (!emissionImpactViewEnabled.value && !newPricingEnabled.value && segmentsUIEnabled.value) cards++;
    if (emissionImpactViewEnabled.value) cards += 2;
    if (billingEnabled.value && !emissionImpactViewEnabled.value) cards++;
    return Math.floor(12 / cards);
});

/**
 * Indicates if segments UI should be shown.
 */
const segmentsUIEnabled = computed<boolean>(() => configStore.state.config.segmentsUIEnabled);

/**
 * Get selected project from store.
 */
const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);

const avoidedSubtitle = computed<string>(() => `By using ${configStore.brandName}`);

/**
 * Returns project's emission impact.
 */
const emission = computed<Emission>(() => projectsStore.state.emission);

/**
 * Returns formatted CO2 estimated info.
 */
const co2Estimated = computed<string>(() => {
    const formatted = getValueAndUnit(Math.round(emission.value.storjImpact));

    return `${formatted.value.toLocaleString()} ${formatted.unit} CO₂e`;
});

/**
 * Returns formatted CO2 save info.
 */
const co2Saved = computed<string>(() => {
    let value = Math.round(emission.value.hyperscalerImpact) - Math.round(emission.value.storjImpact);
    if (value < 0) value = 0;

    const formatted = getValueAndUnit(value);

    return `${formatted.value.toLocaleString()} ${formatted.unit} CO₂e`;
});

/**
 * Returns access grants count from store.
 */
const accessGrantsCount = computed<number>(() => agStore.state.allAGNames.length);

/**
 * Returns access grants count from store.
 */
const bucketsCount = computed<number>(() => bucketsStore.state.allBucketNames.length);

/**
 * Indicates if emission impact view should be shown.
 */
const emissionImpactViewEnabled = computed<boolean>(() => configStore.state.config.emissionImpactViewEnabled);

/**
 * Returns user account tier string.
 */
const paidTierString = computed<string>(() => usersStore.state.user.isPaid ? 'Pro' : 'Free');

/**
 * Returns current limits from store.
 */
const limits = computed<ProjectLimits>(() => projectsStore.state.currentLimits);

/**
 * Returns charts since date from store.
 */
const chartsSinceDate = computed<Date>(() => projectsStore.state.chartDataSince);

/**
 * Returns charts before date from store.
 */
const chartsBeforeDate = computed<Date>(() => projectsStore.state.chartDataBefore);

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
 * Returns adjusted value and unit.
 */
function getValueAndUnit(value: number): ValueUnit {
    const unitUpgradeThreshold = 999999;
    const [newValue, unit] = value > unitUpgradeThreshold ? [value / 1000, 't'] : [value, 'kg'];

    return { value: newValue, unit };
}

/**
 * Used container size recalculation for charts resizing.
 */
function recalculateChartWidth(): void {
    chartWidth.value = chartContainer.value?.$el.getBoundingClientRect().width - 16 || 0;
}

async function fetchData(): Promise<void> {
    const projectID = selectedProject.value.id;

    const promises: Promise<void>[] = [
        bucketsStore.getAllBucketsNames(projectID),
        agStore.getAllAGNames(projectID),
        projectsStore.getDailyProjectData({ since: chartDateRange.value[0], before: chartDateRange.value[chartDateRange.value.length - 1] }),
    ];
    if (emissionImpactViewEnabled.value) {
        promises.push(projectsStore.getEmissionImpact(projectID));
    }

    try {
        await Promise.all(promises);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_USAGE_PAGE);
    }
}

/**
 * Lifecycle hook after initial render.
 */
onMounted(() => {
    window.addEventListener('resize', recalculateChartWidth);
    recalculateChartWidth();

    void fetchData();
});

/**
 * Lifecycle hook before component destruction.
 * Removes event listener on window resizing.
 */
onBeforeUnmount((): void => {
    window.removeEventListener('resize', recalculateChartWidth);
});

watch(() => selectedProject.value.id, () => {
    void fetchData();
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
