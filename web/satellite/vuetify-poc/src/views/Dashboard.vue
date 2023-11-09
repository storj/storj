// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row v-if="promptForPassphrase && !bucketWasCreated" class="mt-10 mb-15">
            <v-col cols="12">
                <p class="text-h5 font-weight-bold">Set an encryption passphrase<br>to start uploading files.</p>
            </v-col>
            <v-col cols="12">
                <v-btn append-icon="mdi-chevron-right" @click="isSetPassphraseDialogShown = true">
                    Set Encryption Passphrase
                </v-btn>
            </v-col>
        </v-row>
        <v-row v-else-if="!promptForPassphrase && !bucketWasCreated && !bucketsCount" class="mt-10 mb-15">
            <v-col cols="12">
                <p class="text-h5 font-weight-bold">Create a bucket to start<br>uploading data in your project.</p>
            </v-col>
            <v-col cols="12">
                <v-btn append-icon="mdi-chevron-right" @click="isCreateBucketDialogShown = true">
                    Create a Bucket
                </v-btn>
            </v-col>
        </v-row>

        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <PageTitleComponent title="Project Overview" />
                <PageSubtitleComponent
                    :subtitle="`Your ${limits.objectCount.toLocaleString()} files are stored in ${limits.segmentCount.toLocaleString()} segments around the world.`"
                    link="https://docs.storj.io/dcs/pricing#per-segment-fee"
                />
            </v-col>
            <v-col v-if="!isPaidTier && billingEnabled" cols="auto">
                <v-btn @click="appStore.toggleUpgradeFlow(true)">
                    Upgrade
                </v-btn>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center mt-2">
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
            <v-col v-if="billingEnabled" cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent icon="card" title="Billing" :subtitle="`${paidTierString} account`" :data="paidTierString" to="/account/billing" />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center">
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    icon="cloud"
                    title="Storage"
                    :progress="storageUsedPercent"
                    :used="`${usedLimitFormatted(limits.storageUsed)} Used`"
                    :limit="`Limit: ${usedLimitFormatted(limits.storageLimit)}`"
                    :available="`${usedLimitFormatted(availableStorage)} Available`"
                    cta="Need more?"
                    @cta-click="onNeedMoreClicked(LimitToChange.Storage)"
                />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    icon="arrow-down"
                    title="Download"
                    :progress="egressUsedPercent"
                    :used="`${usedLimitFormatted(limits.bandwidthUsed)} Used`"
                    :limit="`Limit: ${usedLimitFormatted(limits.bandwidthLimit)} per month`"
                    :available="`${usedLimitFormatted(availableEgress)} Available`"
                    cta="Need more?"
                    @cta-click="onNeedMoreClicked(LimitToChange.Bandwidth)"
                />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" md="6">
                <v-card ref="chartContainer" class="pb-4" variant="flat" :border="true" rounded="xlg">
                    <template #title>
                        <v-card-title class="d-flex align-center">
                            <IconCloud class="mr-2" width="16" height="16" />
                            Storage
                        </v-card-title>
                    </template>
                    <h5 class="pl-4">{{ getDimension(storageUsage) }}</h5>
                    <StorageChart
                        :width="chartWidth"
                        :height="160"
                        :data="storageUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                    />
                </v-card>
            </v-col>
            <v-col cols="12" md="6">
                <v-card class="pb-4" variant="flat" :border="true" rounded="xlg">
                    <template #title>
                        <v-card-title class="d-flex align-center justify-space-between">
                            <v-row class="ma-0 align-center">
                                <IconArrowDown class="mr-2" width="16" height="16" />
                                Download
                            </v-row>
                            <v-row class="ma-0 align-center justify-end">
                                <v-badge dot inline color="#929fb1" />
                                <v-card-text class="pa-0 mx-2 flex-0-0">Download</v-card-text>
                                <v-tooltip width="250" location="bottom">
                                    <template #activator="{ props }">
                                        <v-icon v-bind="props" size="20" icon="mdi-information-outline" />
                                    </template>
                                    <template #default>
                                        <p>
                                            The most recent data points may change as traffic moves from "allocated" to "settled".
                                            <a
                                                class="link"
                                                href="https://docs.storj.io/dcs/pricing#bandwidth-fee"
                                                target="_blank"
                                                rel="noopener noreferrer"
                                            >
                                                Learn more
                                            </a>
                                        </p>
                                    </template>
                                </v-tooltip>
                            </v-row>
                        </v-card-title>
                    </template>
                    <h5 class="pl-4">{{ getDimension(allocatedBandwidthUsage) }}</h5>
                    <BandwidthChart
                        :width="chartWidth"
                        :height="160"
                        :data="allocatedBandwidthUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                    />
                </v-card>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center">
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    icon="globe"
                    title="Segments"
                    :progress="segmentUsedPercent"
                    :used="`${limits.segmentUsed.toLocaleString()} Used`"
                    :limit="`Limit: ${limits.segmentLimit.toLocaleString()}`"
                    :available="`${availableSegment.toLocaleString()} Available`"
                    :cta="!isPaidTier && billingEnabled ? 'Need more?' : 'Learn more'"
                    @cta-click="onSegmentsCTAClicked"
                />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    v-if="billingStore.state.coupon && billingEnabled"
                    icon="check"
                    :title="isFreeTierCoupon ? 'Free Usage' : 'Coupon'"
                    :progress="couponProgress"
                    :used="`${couponProgress}% Used`"
                    :limit="`Included free usage: ${couponValue}`"
                    :available="`${couponRemainingPercent}% Available`"
                    :cta="isFreeTierCoupon ? 'Learn more' : 'View Coupons'"
                    @cta-click="onCouponCTAClicked"
                />
            </v-col>
        </v-row>

        <v-col class="pa-0 mt-6" cols="12">
            <v-card-title class="font-weight-bold pl-0">Buckets</v-card-title>
            <buckets-data-table />
        </v-col>
    </v-container>

    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="limitToChange" />
    <create-bucket-dialog v-model="isCreateBucketDialogShown" />
    <manage-passphrase-dialog v-model="isSetPassphraseDialogShown" is-create />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { VBtn, VCard, VCardTitle, VCardText, VCol, VContainer, VRow, VBadge, VIcon, VTooltip } from 'vuetify/components';
import { ComponentPublicInstance } from '@vue/runtime-core';
import { useRouter } from 'vue-router';

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
import { LocalData } from '@/utils/localData';
import { ProjectMembersPage } from '@/types/projectMembers';
import { AccessGrantsPage } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import CardStatsComponent from '@poc/components/CardStatsComponent.vue';
import UsageProgressComponent from '@poc/components/UsageProgressComponent.vue';
import BandwidthChart from '@/components/project/dashboard/BandwidthChart.vue';
import StorageChart from '@/components/project/dashboard/StorageChart.vue';
import BucketsDataTable from '@poc/components/BucketsDataTable.vue';
import EditProjectLimitDialog from '@poc/components/dialogs/EditProjectLimitDialog.vue';
import CreateBucketDialog from '@poc/components/dialogs/CreateBucketDialog.vue';
import ManagePassphraseDialog from '@poc/components/dialogs/ManagePassphraseDialog.vue';
import IconCloud from '@poc/components/icons/IconCloud.vue';
import IconArrowDown from '@poc/components/icons/IconArrowDown.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();

const notify = useNotify();
const router = useRouter();

const bucketWasCreated = !!LocalData.getBucketWasCreatedStatus();

const chartWidth = ref<number>(0);
const chartContainer = ref<ComponentPublicInstance>();
const isEditLimitDialogShown = ref<boolean>(false);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);
const isCreateBucketDialogShown = ref<boolean>(false);
const isSetPassphraseDialogShown = ref<boolean>(false);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

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
 * Indicates if active coupon is free tier coupon.
 */
const isFreeTierCoupon = computed((): boolean => {
    if (!billingStore.state.coupon) {
        return true;
    }

    const freeTierCouponName = 'Free Tier';

    return billingStore.state.coupon.name.includes(freeTierCouponName);
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
 * Returns user account tier string.
 */
const paidTierString = computed((): string => {
    return isPaidTier.value ? 'Pro' : 'Free';
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
 * Indicates if user should be prompted to enter the project passphrase.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
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
 * Used container size recalculation for charts resizing.
 */
function recalculateChartWidth(): void {
    chartWidth.value = chartContainer.value?.$el.getBoundingClientRect().width - 16 || 0;
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
    if (!isPaidTier.value && billingEnabled.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    limitToChange.value = source;
    isEditLimitDialogShown.value = true;
}

/**
 * Conditionally opens the upgrade dialog or docs link.
 */
function onSegmentsCTAClicked(): void {
    if (!isPaidTier.value && billingEnabled.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    window.open('https://docs.storj.io/dcs/pricing#per-segment-fee', '_blank', 'noreferrer');
}

/**
 * Conditionally opens docs link or navigates to billing overview.
 */
function onCouponCTAClicked(): void {
    if (isFreeTierCoupon.value) {
        window.open('https://docs.storj.io/dcs/pricing#free-tier', '_blank', 'noreferrer');
        return;
    }

    router.push('/account/billing');
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

    let promises: Promise<void | ProjectMembersPage | AccessGrantsPage>[] = [
        projectsStore.getDailyProjectData({ since: past, before: now }),
        projectsStore.getProjectLimits(projectID),
        pmStore.getProjectMembers(FIRST_PAGE, projectID),
        agStore.getAccessGrants(FIRST_PAGE, projectID),
        bucketsStore.getBuckets(FIRST_PAGE, projectID),
    ];

    if (billingEnabled.value) {
        promises = [
            ...promises,
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            billingStore.getCoupon(),
        ];
    }

    try {
        await Promise.all(promises);
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
