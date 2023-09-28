// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <PageTitleComponent title="Project Overview" />
                <PageSubtitleComponent
                    :subtitle="`Your ${limits.objectCount.toLocaleString()} files are stored in ${limits.segmentCount.toLocaleString()} segments around the world.`"
                    link="https://docs.storj.io/dcs/pricing#per-segment-fee"
                />
            </v-col>
            <v-col v-if="!isPaidTier" cols="auto">
                <v-btn @click="appStore.toggleUpgradeFlow(true)">
                    Upgrade plan
                </v-btn>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" md="6">
                <v-card ref="chartContainer" title="Storage" variant="flat" :border="true" rounded="xlg">
                    <h4 class="pl-4">{{ getDimension(storageUsage) }}</h4>
                    <StorageChart
                        :width="chartWidth"
                        :height="170"
                        :data="storageUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                    />
                </v-card>
            </v-col>
            <v-col cols="12" md="6">
                <v-card title="Download" variant="flat" :border="true" rounded="xlg">
                    <h4 class="pl-4">{{ getDimension(allocatedBandwidthUsage) }}</h4>
                    <BandwidthChart
                        :width="chartWidth"
                        :height="170"
                        :data="allocatedBandwidthUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                    />
                </v-card>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="file" title="Files" subtitle="Project files" :data="limits.objectCount.toLocaleString()" to="buckets" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="globe" title="Segments" subtitle="All file pieces" :data="limits.segmentCount.toLocaleString()" to="buckets" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="bucket" title="Buckets" subtitle="Project buckets" :data="bucketsCount.toLocaleString()" to="buckets" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="access" title="Access" subtitle="Project accesses" :data="accessGrantsCount.toLocaleString()" to="access" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="team" title="Team" subtitle="Project members" :data="teamSize.toLocaleString()" to="team" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="card" title="Billing" :subtitle="`${paidTierString} account`" :data="paidTierString" to="/account/billing" />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center">
            <v-col cols="12" md="6">
                <UsageProgressComponent icon="cloud" title="Storage" :progress="storageUsedPercent" :used="`${usedLimitFormatted(limits.storageUsed)} Used`" :limit="`Limit: ${usedLimitFormatted(limits.storageLimit)}`" :available="`${usedLimitFormatted(availableStorage)} Available`" cta="Need more?" @cta-click="onNeedMoreClicked(LimitToChange.Storage)" />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent icon="arrow-down" title="Download" :progress="egressUsedPercent" :used="`${usedLimitFormatted(limits.bandwidthUsed)} Used`" :limit="`Limit: ${usedLimitFormatted(limits.bandwidthLimit)}`" :available="`${usedLimitFormatted(availableEgress)} Available`" cta="Need more?" @cta-click="onNeedMoreClicked(LimitToChange.Bandwidth)" />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent icon="globe" title="Segments" :progress="segmentUsedPercent" :used="`${limits.segmentUsed.toLocaleString()} Used`" :limit="`Limit: ${limits.segmentLimit.toLocaleString()}`" :available="`${availableSegment.toLocaleString()} Available`" cta="Learn more" />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent v-if="billingStore.state.coupon" icon="check" :title="billingStore.state.coupon.name" :progress="couponProgress" :used="`${usedLimitFormatted(limits.storageUsed + limits.bandwidthUsed)} Used`" :limit="`Limit: ${couponValue}`" :available="`${couponRemainingPercent}% Available`" cta="" />
            </v-col>
        </v-row>
        <v-col class="pa-0 mt-6" cols="12">
            <v-card-title class="font-weight-bold pl-0">Buckets</v-card-title>
            <buckets-data-table />
        </v-col>
    </v-container>

    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="limitToChange" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { VBtn, VCard, VCardTitle, VCol, VContainer, VRow } from 'vuetify/components';
import { ComponentPublicInstance } from '@vue/runtime-core';

import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { DataStamp, LimitToChange, Project, ProjectLimits } from '@/types/projects';
import { Dimensions, Size } from '@/utils/bytesSize';
import { ChartUtils } from '@/utils/chart';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@poc/store/appStore';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import CardStatsComponent from '@poc/components/CardStatsComponent.vue';
import UsageProgressComponent from '@poc/components/UsageProgressComponent.vue';
import BandwidthChart from '@/components/project/dashboard/BandwidthChart.vue';
import StorageChart from '@/components/project/dashboard/StorageChart.vue';
import BucketsDataTable from '@poc/components/BucketsDataTable.vue';
import EditProjectLimitDialog from '@poc/components/dialogs/EditProjectLimitDialog.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();

const chartWidth = ref<number>(0);
const chartContainer = ref<ComponentPublicInstance>();
const isEditLimitDialogShown = ref<boolean>(false);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);

/**
 * Returns percent of coupon used.
 */
const couponProgress = computed((): number => {
    if (!billingStore.state.coupon) {
        return 0;
    }
    const charges = billingStore.state.projectCharges.getPrice();
    const couponValue = billingStore.state.coupon.amountOff;
    if (charges > couponValue) {
        return 100;
    }
    return Math.round(charges / couponValue * 100);
});

/**
 * Returns coupon value.
 */
const couponValue = computed((): string => {
    return billingStore.state.coupon?.amountOff ? '$' + (billingStore.state.coupon.amountOff * 0.01).toLocaleString() : billingStore.state.coupon?.percentOff.toLocaleString() + '%';
});

/**
 * Returns percent of coupon value remaining.
 */
const couponRemainingPercent = computed((): number => {
    return 100 - couponProgress.value;
});

/**
 * Whether the user is in paid tier.
 */
const isPaidTier = computed((): boolean => {
    return usersStore.state.user.paidTier;
});

/**
 * Returns formatted amount.
 */
function usedLimitFormatted(value: number): string {
    return formattedValue(new Size(value, 2));
}

/**
 * Formats value to needed form and returns it.
 */
function formattedValue(value: Size): string {
    switch (value.label) {
    case Dimensions.Bytes:
        return '0';
    default:
        return `${value.formattedBytes.replace(/\.0+$/, '')}${value.label}`;
    }
}

/**
 * Returns user account tier string.
 */
const paidTierString = computed((): string => {
    return usersStore.state.user.paidTier ? 'Pro' : 'Free';
});

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Returns remaining segments available.
 */
const availableSegment = computed((): number => {
    return projectsStore.state.currentLimits.segmentLimit - projectsStore.state.currentLimits.segmentUsed;
});

/**
 * Returns percentage of segment limit used.
 */
const segmentUsedPercent = computed((): number => {
    return projectsStore.state.currentLimits.segmentUsed/projectsStore.state.currentLimits.segmentLimit * 100;
});

/**
 * Returns remaining egress available.
 */
const availableEgress = computed((): number => {
    return projectsStore.state.currentLimits.bandwidthLimit - projectsStore.state.currentLimits.bandwidthUsed;
});

/**
 * Returns percentage of egress limit used.
 */
const egressUsedPercent = computed((): number => {
    return projectsStore.state.currentLimits.bandwidthUsed/projectsStore.state.currentLimits.bandwidthLimit * 100;
});

/**
 * Returns remaining storage available.
 */
const availableStorage = computed((): number => {
    return projectsStore.state.currentLimits.storageLimit - projectsStore.state.currentLimits.storageUsed;
});

/**
 * Returns percentage of storage limit used.
 */
const storageUsedPercent = computed((): number => {
    return projectsStore.state.currentLimits.storageUsed/projectsStore.state.currentLimits.storageLimit * 100;
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
 * Conditionally opens the upgrade dialog
 * or the edit limit dialog.
 */
function onNeedMoreClicked(source: LimitToChange): void {
    if (!usersStore.state.user.paidTier) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    limitToChange.value = source;
    isEditLimitDialogShown.value = true;
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
            billingStore.getCoupon(),
            pmStore.getProjectMembers(FIRST_PAGE, projectID),
            agStore.getAccessGrants(FIRST_PAGE, projectID),
            bucketsStore.getBuckets(FIRST_PAGE, projectID),
        ]);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
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
