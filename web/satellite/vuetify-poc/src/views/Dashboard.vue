// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Project Overview" />
        <PageSubtitleComponent :subtitle="`Your ${limits.objectCount.toLocaleString()} files are stored in ${limits.segmentCount.toLocaleString()} segments around the world.`" />

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" md="6">
                <v-card ref="chartContainer" title="Storage" variant="flat" :border="true" rounded="xlg">
                    <template v-if="!isDataFetching">
                        <h4 class="pl-4">{{ getDimension(storageUsage) }}</h4>
                        <StorageChart
                            :width="chartWidth"
                            :height="170"
                            :data="storageUsage"
                            :since="chartsSinceDate"
                            :before="chartsBeforeDate"
                        />
                    </template>
                </v-card>
            </v-col>
            <v-col cols="12" md="6">
                <v-card title="Bandwidth" variant="flat" :border="true" rounded="xlg">
                    <template v-if="!isDataFetching">
                        <h4 class="pl-4">{{ getDimension([...settledBandwidthUsage, ...allocatedBandwidthUsage]) }}</h4>
                        <BandwidthChart
                            :width="chartWidth"
                            :height="170"
                            :settled-data="settledBandwidthUsage"
                            :allocated-data="allocatedBandwidthUsage"
                            :since="chartsSinceDate"
                            :before="chartsBeforeDate"
                            is-vuetify
                        />
                    </template>
                </v-card>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Files" subtitle="Project files" :data="limits.objectCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Segments" subtitle="All file pieces" :data="limits.segmentCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Buckets" subtitle="Project buckets" :data="bucketsCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Access" subtitle="Project accesses" :data="accessGrantsCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Team" subtitle="Project members" :data="teamSize.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Billing" subtitle="Free account" data="Free" />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center">
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Storage" :progress="40" used="10 GB Used" limit="Limit: 25GB" available="15 GB Available" cta="Need more?" />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Download" :progress="60" used="15 GB Used" limit="Limit: 25GB per month" available="10 GB Available" cta="Need more?" />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Segments" :progress="12" used="1,235 Used" limit="Limit: 10,000" available="8,765 Available" cta="Learn more" />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Free Tier" :progress="50" used="10 GB Used" limit="Limit: $1.65" available="50% Available" cta="Upgrade to Pro" />
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { VContainer, VRow, VCol, VCard } from 'vuetify/components';
import { ComponentPublicInstance } from '@vue/runtime-core';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { DataStamp, Project, ProjectLimits } from '@/types/projects';
import { Dimensions, Size } from '@/utils/bytesSize';
import { ChartUtils } from '@/utils/chart';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import CardStatsComponent from '@poc/components/CardStatsComponent.vue';
import UsageProgressComponent from '@poc/components/UsageProgressComponent.vue';
import BandwidthChart from '@/components/project/dashboard/BandwidthChart.vue';
import StorageChart from '@/components/project/dashboard/StorageChart.vue';

const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();

const isDataFetching = ref<boolean>(true);
const chartWidth = ref<number>(0);
const chartContainer = ref<ComponentPublicInstance>();

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Get selected project from store.
 */
const selectedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

/**
 * Returns current team size from store.
 */
const teamSize = computed((): number => {
    return pmStore.state.page.totalCount;
});

/**
 * Returns access grants count from store.
 */
const accessGrantsCount = computed((): number => {
    return agStore.state.page.totalCount;
});

/**
 * Returns access grants count from store.
 */
const bucketsCount = computed((): number => {
    return bucketsStore.state.page.totalCount;
});

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
 * Returns storage chart data from store.
 */
const storageUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.storageChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns settled bandwidth chart data from store.
 */
const settledBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.settledBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns allocated bandwidth chart data from store.
 */
const allocatedBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.allocatedBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Used container size recalculation for charts resizing.
 */
function recalculateChartWidth(): void {
    chartWidth.value = chartContainer.value?.$el.getBoundingClientRect().width || 0;
}

/**
 * Returns dimension for given data values.
 */
function getDimension(dataStamps: DataStamp[]): Dimensions {
    const maxValue = Math.max(...dataStamps.map(s => s.value));
    return new Size(maxValue).label;
}

/**
 * Lifecycle hook after initial render.
 * Fetches project limits.
 */
onMounted(async (): Promise<void> => {
    const projectID = selectedProject.value.id;
    const FIRST_PAGE = 1;

    window.addEventListener('resize', recalculateChartWidth);
    recalculateChartWidth();

    const now = new Date();
    const past = new Date();
    past.setDate(past.getDate() - 30);

    try {
        await Promise.all([
            projectsStore.getDailyProjectData({ since: past, before: now }),
            projectsStore.getProjectLimits(projectID),
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            pmStore.getProjectMembers(FIRST_PAGE, projectID),
            agStore.getAccessGrants(FIRST_PAGE, projectID),
            bucketsStore.getBuckets(FIRST_PAGE, projectID),
        ]);
    } catch (error) { /* empty */ } finally {
        isDataFetching.value = false;
    }
});

/**
 * Lifecycle hook before component destruction.
 * Removes event listener on window resizing.
 */
onBeforeUnmount((): void => {
    window.removeEventListener('resize', recalculateChartWidth);
});
</script>
