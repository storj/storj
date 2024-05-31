// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-15">
        <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

        <next-steps-container />

        <low-token-balance-banner
            v-if="isLowBalance && billingEnabled"
            cta-label="Go to billing"
            @click="redirectToBilling"
        />
        <limit-warning-banners v-if="billingEnabled" />
        <versioning-beta-banner v-if="!versioningBetaBannerDismissed" />

        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <PageTitleComponent title="Project dashboard" />
                <PageSubtitleComponent
                    subtitle="View your project statistics, check daily usage, and set project limits."
                    link="https://docs.storj.io/support/projects"
                />
            </v-col>
            <v-col cols="auto" class="pt-0 mt-0 pt-md-5">
                <v-btn v-if="!isPaidTier && billingEnabled" variant="outlined" color="default" @click="appStore.toggleUpgradeFlow(true)">
                    <IconUpgrade size="16" class="mr-2" />
                    Upgrade
                </v-btn>
            </v-col>
        </v-row>

        <team-passphrase-banner v-if="isTeamPassphraseBanner" />

        <v-row class="d-flex align-center mt-2">
            <v-col cols="6" md="4" lg="2">
                <CardStatsComponent title="Files" subtitle="Project total" :data="limits.objectCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col v-if="!emissionImpactViewEnabled" cols="6" md="4" lg="2">
                <CardStatsComponent title="Segments" subtitle="All file pieces" :data="limits.segmentCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col cols="6" md="4" lg="2">
                <CardStatsComponent title="Buckets" subtitle="In this project" :data="bucketsCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col cols="6" md="4" lg="2">
                <CardStatsComponent title="Access Keys" subtitle="Total keys" :data="accessGrantsCount.toLocaleString()" :to="ROUTES.Access.path" />
            </v-col>
            <v-col cols="6" md="4" lg="2">
                <CardStatsComponent title="Team" subtitle="Project members" :data="teamSize.toLocaleString()" :to="ROUTES.Team.path" />
            </v-col>
            <template v-if="emissionImpactViewEnabled">
                <v-col cols="12" sm="6" md="4" lg="2">
                    <emissions-dialog />
                    <v-tooltip
                        activator="parent"
                        location="top"
                        offset="-20"
                        opacity="80"
                    >
                        Click to learn more
                    </v-tooltip>
                    <CardStatsComponent title="CO₂ Estimated" subtitle="For this project" :data="co2Estimated" link />
                </v-col>
                <v-col cols="12" sm="6" md="4" lg="2">
                    <emissions-dialog />
                    <v-tooltip
                        activator="parent"
                        location="top"
                        offset="-20"
                        opacity="80"
                    >
                        Click to learn more
                    </v-tooltip>
                    <CardStatsComponent title="CO₂ Avoided" subtitle="By using Storj" :data="co2Saved" color="success" link />
                </v-col>
            </template>
            <v-col v-if="billingEnabled && !emissionImpactViewEnabled" cols="6" md="4" lg="2">
                <CardStatsComponent title="Billing" :subtitle="`${paidTierString} account`" :data="paidTierString" :to="ROUTES.Account.with(ROUTES.Billing).path" />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mb-5">
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    icon="cloud"
                    title="Storage"
                    :progress="storageUsedPercent"
                    :used="`${usedLimitFormatted(limits.storageUsed)} Used`"
                    :limit="`Limit: ${usedLimitFormatted(limits.storageLimit)}`"
                    :available="`${usedLimitFormatted(availableStorage)} Available`"
                    :cta="getCTALabel(storageUsedPercent)"
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
                    :cta="getCTALabel(egressUsedPercent)"
                    extra-info="The download bandwidth usage is only for the current billing period of one month."
                    @cta-click="onNeedMoreClicked(LimitToChange.Bandwidth)"
                />
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    icon="globe"
                    title="Segments"
                    :progress="segmentUsedPercent"
                    :used="`${limits.segmentUsed.toLocaleString()} Used`"
                    :limit="`Limit: ${limits.segmentLimit.toLocaleString()}`"
                    :available="`${availableSegment.toLocaleString()} Available`"
                    :cta="getCTALabel(segmentUsedPercent, true)"
                    @cta-click="onSegmentsCTAClicked"
                >
                    <template #extraInfo>
                        <p>
                            Segments are the encrypted parts of an uploaded file.
                            <a
                                class="link"
                                href="https://docs.storj.io/dcs/pricing#per-segment-fee"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Learn more
                            </a>
                        </p>
                    </template>
                </UsageProgressComponent>
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent
                    v-if="isCouponCard"
                    icon="check"
                    :title="isFreeTierCoupon ? 'Free Usage' : 'Coupon'"
                    :progress="couponProgress"
                    :used="`${couponProgress}% Used`"
                    :limit="`Included free usage: ${couponValue}`"
                    :available="`${couponRemainingPercent}% Available`"
                    :cta="isFreeTierCoupon ? 'Learn more' : 'View Coupons'"
                    @cta-click="onCouponCTAClicked"
                />
                <UsageProgressComponent
                    v-else
                    icon="bucket"
                    title="Buckets"
                    :progress="bucketsUsedPercent"
                    :used="`${limits.bucketsUsed.toLocaleString()} Used`"
                    :limit="`Limit: ${limits.bucketsLimit.toLocaleString()}`"
                    :available="`${availableBuckets.toLocaleString()} Available`"
                    cta="Need more?"
                    @cta-click="onBucketsCTAClicked"
                />
            </v-col>
        </v-row>

        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <v-card-title class="font-weight-bold pl-0">Daily usage</v-card-title>
                <p class="text-medium-emphasis">
                    Select date range to view daily usage statistics.
                </p>
            </v-col>
            <v-col cols="auto" class="pt-0 mt-0 pt-md-5">
                <v-btn prepend-icon="$calendar" variant="outlined" color="default" @click="isDatePicker = true">
                    {{ dateRangeLabel }}
                </v-btn>
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2 mb-5">
            <v-col cols="12" md="6">
                <v-card ref="chartContainer" class="pb-4">
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
                <v-card class="pb-4">
                    <template #title>
                        <v-card-title class="d-flex align-center justify-space-between">
                            <v-row class="ma-0 align-center">
                                <IconArrowDown class="mr-2" width="16" height="16" />
                                Download
                                <v-tooltip width="250" location="bottom">
                                    <template #activator="{ props }">
                                        <v-icon v-bind="props" size="16" :icon="mdiInformationOutline" class="ml-2 text-medium-emphasis" />
                                    </template>
                                    <template #default>
                                        <p>
                                            The most recent data may change as download moves from "allocated" to "settled".
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

        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <v-card-title class="font-weight-bold pl-0">
                    Storage buckets
                </v-card-title>
                <p class="text-medium-emphasis">
                    Buckets are where you upload and organize your data.
                </p>
            </v-col>
            <v-col cols="auto" class="pt-0 mt-0 pt-md-5">
                <v-btn
                    variant="outlined"
                    color="default"
                    @click="onCreateBucket"
                >
                    <IconCirclePlus class="mr-2" />
                    New Bucket
                </v-btn>
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <buckets-data-table />
            </v-col>
        </v-row>
    </v-container>

    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="limitToChange" />
    <create-bucket-dialog v-model="isCreateBucketDialogShown" />
    <CreateBucketDialog v-model="isCreateBucketDialogOpen" />

    <v-overlay v-model="isDatePicker" class="align-center justify-center">
        <v-date-picker
            v-model="datePickerModel"
            multiple
            show-adjacent-months
            title="Select Date Range"
            header="Daily Usage"
            :disabled="isLoading"
        />
    </v-overlay>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardTitle,
    VCol,
    VContainer,
    VRow,
    VIcon,
    VTooltip,
    VDatePicker,
    VOverlay,
} from 'vuetify/components';
import { ComponentPublicInstance } from '@vue/runtime-core';
import { useRouter } from 'vue-router';
import { mdiInformationOutline } from '@mdi/js';

import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { DataStamp, Emission, LimitToChange, Project, ProjectLimits } from '@/types/projects';
import { Dimensions, Size } from '@/utils/bytesSize';
import { ChartUtils } from '@/utils/chart';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { ProjectMembersPage } from '@/types/projectMembers';
import { AccessGrantsPage } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';
import { ROUTES } from '@/router';
import { AccountBalance, CreditCard } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useTrialCheck } from '@/composables/useTrialCheck';
import { Time } from '@/utils/time';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';
import UsageProgressComponent from '@/components/UsageProgressComponent.vue';
import BandwidthChart from '@/components/BandwidthChart.vue';
import StorageChart from '@/components/StorageChart.vue';
import BucketsDataTable from '@/components/BucketsDataTable.vue';
import EditProjectLimitDialog from '@/components/dialogs/EditProjectLimitDialog.vue';
import CreateBucketDialog from '@/components/dialogs/CreateBucketDialog.vue';
import IconCloud from '@/components/icons/IconCloud.vue';
import IconArrowDown from '@/components/icons/IconArrowDown.vue';
import LimitWarningBanners from '@/components/LimitWarningBanners.vue';
import LowTokenBalanceBanner from '@/components/LowTokenBalanceBanner.vue';
import IconUpgrade from '@/components/icons/IconUpgrade.vue';
import IconCirclePlus from '@/components/icons/IconCirclePlus.vue';
import NextStepsContainer from '@/components/onboarding/NextStepsContainer.vue';
import TeamPassphraseBanner from '@/components/TeamPassphraseBanner.vue';
import EmissionsDialog from '@/components/dialogs/EmissionsDialog.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';
import VersioningBetaBanner from '@/components/VersioningBetaBanner.vue';

type ValueUnit = {
    value: number
    unit: string
}

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
const isLowBalance = useLowTokenBalance();
const { isLoading, withLoading } = useLoading();
const { isTrialExpirationBanner, isUserProjectOwner, isExpired, withTrialCheck } = useTrialCheck();

const chartWidth = ref<number>(0);
const chartContainer = ref<ComponentPublicInstance>();
const isEditLimitDialogShown = ref<boolean>(false);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);
const isCreateBucketDialogShown = ref<boolean>(false);
const isCreateBucketDialogOpen = ref<boolean>(false);
const isDatePicker = ref<boolean>(false);
const datePickerModel = ref<Date[]>([]);

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
 * Returns formatted date range string.
 */
const dateRangeLabel = computed((): string => {
    if (chartsSinceDate.value.getTime() === chartsBeforeDate.value.getTime()) {
        return Time.formattedDate(chartsSinceDate.value);
    }

    return `${Time.formattedDate(chartsSinceDate.value)} - ${Time.formattedDate(chartsBeforeDate.value)}`;
});

/**
 * Indicates if billing coupon card should be shown.
 */
const isCouponCard = computed<boolean>(() => {
    return billingStore.state.coupon !== null &&
        billingEnabled.value &&
        !isPaidTier.value &&
        selectedProject.value.ownerId === usersStore.state.user.id;
});

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user.hasVarPartner));

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
 * Whether project members passphrase banner should be shown.
 */
const isTeamPassphraseBanner = computed<boolean>(() => {
    return !usersStore.state.settings.noticeDismissal.projectMembersPassphrase && teamSize.value > 1;
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
    const diff = limits.value.segmentLimit - limits.value.segmentUsed;
    return diff < 0 ? 0 : diff;
});

/**
 * Returns percentage of segment limit used.
 */
const segmentUsedPercent = computed((): number => {
    return limits.value.segmentUsed / limits.value.segmentLimit * 100;
});

/**
 * Returns remaining egress available.
 */
const availableEgress = computed((): number => {
    const diff = limits.value.bandwidthLimit - limits.value.bandwidthUsed;
    return diff < 0 ? 0 : diff;
});

/**
 * Returns percentage of egress limit used.
 */
const egressUsedPercent = computed((): number => {
    return limits.value.bandwidthUsed / limits.value.bandwidthLimit * 100;
});

/**
 * Returns remaining storage available.
 */
const availableStorage = computed((): number => {
    const diff = limits.value.storageLimit - limits.value.storageUsed;
    return diff < 0 ? 0 : diff;
});

/**
 * Returns percentage of storage limit used.
 */
const storageUsedPercent = computed((): number => {
    return limits.value.storageUsed / limits.value.storageLimit * 100;
});

/**
 * Returns percentage of buckets limit used.
 */
const bucketsUsedPercent = computed((): number => {
    return limits.value.bucketsUsed / limits.value.bucketsLimit * 100;
});

/**
 * Returns remaining buckets available.
 */
const availableBuckets = computed((): number => {
    const diff = limits.value.bucketsLimit - limits.value.bucketsUsed;
    return diff < 0 ? 0 : diff;
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
 * Indicates if emission impact view should be shown.
 */
const emissionImpactViewEnabled = computed<boolean>(() => {
    return configStore.state.config.emissionImpactViewEnabled;
});

/**
 * Returns project's emission impact.
 */
const emission = computed<Emission>(()  => {
    return projectsStore.state.emission;
});

/**
 * Whether the user has dismissed the versioning beta banner.
 */
const versioningBetaBannerDismissed = computed(() => !!usersStore.noticeDismissal?.versioningBetaBanner);

/**
 * Returns adjusted value and unit.
 */
function getValueAndUnit(value: number): ValueUnit {
    const unitUpgradeThreshold = 999999;
    const [newValue, unit] = value > unitUpgradeThreshold ? [value / 1000, 't'] : [value, 'kg'];

    return { value: newValue, unit };
}

/**
 * Starts create bucket flow if user's free trial is not expired.
 */
function onCreateBucket(): void {
    withTrialCheck(() => {
        isCreateBucketDialogOpen.value = true;
    });
}

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
    const filteredData = dataStamps.filter(s => !!s);
    const maxValue = Math.max(...filteredData.map(s => s.value));
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
 * Returns CTA label based on paid tier status and current usage.
 */
function getCTALabel(usage: number, isSegment = false): string {
    if (!isPaidTier.value && billingEnabled.value) {
        if (usage >= 100) {
            return 'Upgrade now';
        }
        if (usage >= 80) {
            return 'Upgrade';
        }
        return 'Need more?';
    }

    if (isSegment) return 'Learn more';

    if (usage >= 80) {
        return 'Increase limits';
    }
    return 'Edit Limit';
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

    redirectToBilling();
}

/**
 * Opens limit increase request link in a new tab.
 */
function onBucketsCTAClicked(): void {
    if (!isPaidTier.value && billingEnabled.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    window.open(configStore.state.config.projectLimitsIncreaseRequestURL, '_blank', 'noreferrer');
}

/**
 * Redirects to Billing Page tab.
 */
function redirectToBilling(): void {
    router.push({ name: ROUTES.Billing.name });
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
    past.setDate(past.getDate() - 7);

    // Truncate dates to hours only.
    now.setMinutes(0, 0, 0);
    past.setMinutes(0, 0, 0);

    const promises: Promise<void | ProjectMembersPage | AccessGrantsPage | AccountBalance | CreditCard[]>[] = [
        projectsStore.getDailyProjectData({ since: past, before: now }),
        projectsStore.getProjectLimits(projectID),
        pmStore.getProjectMembers(FIRST_PAGE, projectID),
        agStore.getAccessGrants(FIRST_PAGE, projectID),
        bucketsStore.getBuckets(FIRST_PAGE, projectID),
    ];

    if (emissionImpactViewEnabled.value) {
        promises.push(projectsStore.getEmissionImpact(projectID));
    }

    if (billingEnabled.value) {
        promises.push(
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            billingStore.getBalance(),
            billingStore.getCreditCards(),
            billingStore.getCoupon(),
        );
    }

    if (configStore.state.config.nativeTokenPaymentsEnabled && billingEnabled.value) {
        promises.push(billingStore.getNativePaymentsHistory());
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

watch(isDatePicker, () => {
    datePickerModel.value = [];
});

watch(datePickerModel, async () => {
    if (datePickerModel.value.length !== 2) return;

    await withLoading(async () => {
        let [startDate, endDate] = datePickerModel.value;
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

    isDatePicker.value = false;
});
</script>
