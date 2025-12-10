// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <announcement-banner />

        <minimum-charge-banner v-if="billingEnabled" />

        <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

        <card-expire-banner />

        <next-steps-container />

        <low-token-balance-banner
            v-if="isLowBalance && billingEnabled"
            cta-label="Go to billing"
            @click="redirectToBilling"
        />
        <limit-warning-banners v-if="billingEnabled" />

        <v-row align="center" justify="space-between">
            <v-col cols="12" md="auto">
                <v-chip v-if="selectedProject.isClassic" variant="tonal" color="warning" size="large" class="font-weight-bold">
                    Classic
                    <v-tooltip activator="parent" location="top">Pricing from before Nov 2025.</v-tooltip>
                </v-chip>
                <PageTitleComponent
                    title="Project Dashboard"
                    extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
                />
                <PageSubtitleComponent
                    subtitle="View your project statistics, check daily usage, and set project limits."
                    link="https://docs.storj.io/support/projects"
                />
            </v-col>
            <v-col cols="auto" class="pt-0 mt-0 pt-md-5">
                <v-btn v-if="isUserProjectOwner && !isPaidTier && billingEnabled" variant="outlined" color="default" :prepend-icon="CircleArrowUp" @click="appStore.toggleUpgradeFlow(true)">
                    Upgrade
                </v-btn>
            </v-col>
        </v-row>

        <team-passphrase-banner v-if="isTeamPassphraseBanner" />

        <v-row align="center" justify="center" class="mt-2">
            <v-col cols="6" md="4" :lg="statsRowLgColSize">
                <CardStatsComponent
                    title="Objects"
                    subtitle="Project total"
                    :data="limits.objectCount.toLocaleString()"
                    :to="ROUTES.Buckets.path"
                    color="info"
                    extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
                />
            </v-col>
            <v-col v-if="!emissionImpactViewEnabled && !newPricingEnabled" cols="6" md="4" :lg="statsRowLgColSize">
                <CardStatsComponent title="Segments" color="info" subtitle="All object pieces" :data="limits.segmentCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col cols="6" md="4" :lg="statsRowLgColSize">
                <CardStatsComponent title="Buckets" color="info" subtitle="In this project" :data="bucketsCount.toLocaleString()" :to="ROUTES.Buckets.path" />
            </v-col>
            <v-col cols="6" md="4" :lg="statsRowLgColSize">
                <CardStatsComponent title="Access Keys" color="info" subtitle="Total keys" :data="accessGrantsCount.toLocaleString()" :to="ROUTES.Access.path" />
            </v-col>
            <v-col cols="6" md="4" :lg="statsRowLgColSize">
                <CardStatsComponent title="Team" color="info" subtitle="Project members" :data="teamSize.toLocaleString()" :to="ROUTES.Team.path" />
            </v-col>
            <template v-if="emissionImpactViewEnabled">
                <v-col cols="12" sm="6" md="4" :lg="statsRowLgColSize">
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
                <v-col cols="12" sm="6" md="4" :lg="statsRowLgColSize">
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
            <v-col v-if="billingEnabled && !emissionImpactViewEnabled" cols="6" md="4" :lg="statsRowLgColSize">
                <CardStatsComponent title="Billing" :subtitle="`${paidTierString} account`" :data="paidTierString" :to="ROUTES.Account.with(ROUTES.Billing).path" />
            </v-col>
        </v-row>

        <v-row align="center" justify="center">
            <v-col cols="12" md="6" :xl="usageRowXlColSize">
                <UsageProgressComponent
                    icon="storage"
                    title="Storage"
                    :progress="storageUsedPercent"
                    :used="`${usedLimitFormatted(limits.storageUsed)} Used`"
                    :limit="storageLimitTxt"
                    :available="storageAvailableTxt"
                    :cta="storageCTA"
                    :no-limit="noLimitsUiEnabled && ownerHasPaidPrivileges && !limits.userSetStorageLimit"
                    extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
                    @cta-click="onNeedMoreClicked(LimitToChange.Storage)"
                />
            </v-col>
            <v-col cols="12" md="6" :xl="usageRowXlColSize">
                <UsageProgressComponent
                    icon="download"
                    title="Download"
                    :progress="egressUsedPercent"
                    :used="`${usedLimitFormatted(limits.bandwidthUsed)} Used`"
                    :limit="bandwidthLimitTxt"
                    :available="bandwidthAvailableTxt"
                    :cta="bandwidthCTA"
                    :no-limit="noLimitsUiEnabled && ownerHasPaidPrivileges && !limits.userSetBandwidthLimit"
                    extra-info="The download bandwidth usage is only for the current billing period of one month."
                    @cta-click="onNeedMoreClicked(LimitToChange.Bandwidth)"
                />
            </v-col>
            <v-col v-if="!newPricingEnabled" cols="12" md="6" :xl="usageRowXlColSize">
                <UsageProgressComponent
                    icon="segments"
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
                            Segments are the encrypted parts of an uploaded object.
                            <a
                                class="link"
                                href="https://storj.dev/dcs/pricing/legacy#segment-fees"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Learn more
                            </a>
                        </p>
                    </template>
                </UsageProgressComponent>
            </v-col>
            <v-col v-if="isCouponCard || !newPricingEnabled" cols="12" md="6" :xl="usageRowXlColSize">
                <UsageProgressComponent
                    v-if="isCouponCard"
                    icon="coupon"
                    :title="isFreeTierCoupon ? 'Free Usage' : 'Coupon'"
                    :progress="couponProgress"
                    :used="`${couponProgress}% Used`"
                    :limit="`Included free usage: ${couponValue}`"
                    :available="`${couponRemainingPercent}% Available`"
                    :hide-cta="!isUserProjectOwner"
                    :cta="isFreeTierCoupon ? 'Learn more' : 'View Coupons'"
                    @cta-click="onCouponCTAClicked"
                />
                <UsageProgressComponent
                    v-else-if="!newPricingEnabled"
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
                <v-card-title class="font-weight-bold pl-0">
                    Storage Buckets
                    <v-tooltip width="240" location="bottom">
                        <template #activator="activator">
                            <v-icon v-bind="activator.props" size="14" :icon="Info" color="info" class="ml-1" />
                        </template>
                        <template #default>
                            <p>Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected.</p>
                        </template>
                    </v-tooltip>
                </v-card-title>
                <p class="text-medium-emphasis">
                    Buckets are where you upload and organize your data.
                </p>
            </v-col>
            <v-col cols="auto" class="pt-0 mt-0 pt-md-5">
                <v-btn
                    variant="outlined"
                    color="default"
                    :prepend-icon="CirclePlus"
                    @click="onCreateBucket"
                >
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
    <CreateBucketDialog v-model="isCreateBucketDialogOpen" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import {
    VBtn,
    VCardTitle,
    VCol,
    VContainer,
    VRow,
    VIcon,
    VTooltip,
    VChip,
} from 'vuetify/components';
import { useRouter } from 'vue-router';
import { Info, CirclePlus, CircleArrowUp } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { Emission, LimitToChange, Project, ProjectLimits } from '@/types/projects';
import { Dimensions, Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useAppStore } from '@/store/modules/appStore';
import { ProjectMembersPage, ProjectRole } from '@/types/projectMembers';
import { AccessGrantsPage } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';
import { ROUTES } from '@/router';
import { AccountBalance, CreditCard } from '@/types/payments';
import { usePreCheck } from '@/composables/usePreCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';
import UsageProgressComponent from '@/components/UsageProgressComponent.vue';
import BucketsDataTable from '@/components/BucketsDataTable.vue';
import EditProjectLimitDialog from '@/components/dialogs/EditProjectLimitDialog.vue';
import CreateBucketDialog from '@/components/dialogs/CreateBucketDialog.vue';
import LimitWarningBanners from '@/components/LimitWarningBanners.vue';
import LowTokenBalanceBanner from '@/components/LowTokenBalanceBanner.vue';
import NextStepsContainer from '@/components/onboarding/NextStepsContainer.vue';
import TeamPassphraseBanner from '@/components/TeamPassphraseBanner.vue';
import EmissionsDialog from '@/components/dialogs/EmissionsDialog.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';
import CardExpireBanner from '@/components/CardExpireBanner.vue';
import MinimumChargeBanner from '@/components/MinimumChargeBanner.vue';
import AnnouncementBanner from '@/components/AnnouncementBanner.vue';

type ValueUnit = {
    value: number
    unit: string
};

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();

const notify = useNotify();
const router = useRouter();
const isLowBalance = useLowTokenBalance();
const { isTrialExpirationBanner, isUserProjectOwner, isExpired, withTrialCheck, withManagedPassphraseCheck } = usePreCheck();

const isEditLimitDialogShown = ref<boolean>(false);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);
const isCreateBucketDialogOpen = ref<boolean>(false);

const avoidedSubtitle = computed<string>(() => `By using ${configStore.brandName}`);

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
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

/**
 * Whether this project has new pricing.
 */
const newPricingEnabled = computed<boolean>(() => {
    if (!billingEnabled.value) return false;
    return configStore.getProjectHasNewPricing(selectedProject.value.createdAt);
});

/**
 * Calculates usage row column size based on enabled cards.
 */
const usageRowXlColSize = computed(() => {
    let cards = 4;
    if (newPricingEnabled.value) cards--;
    if (!isCouponCard.value && newPricingEnabled.value) cards--;
    return Math.floor(12 / cards);
});

/**
 * Calculates stats row column size based on enabled cards.
 */
const statsRowLgColSize = computed(() => {
    let cards = 4;
    if (!emissionImpactViewEnabled.value && !newPricingEnabled.value) cards++;
    if (emissionImpactViewEnabled.value) cards += 2;
    if (billingEnabled.value && !emissionImpactViewEnabled.value) cards++;
    return Math.floor(12 / cards);
});

/**
 * Returns percent of coupon used.
 */
const couponProgress = computed((): number => {
    if (!billingStore.state.coupon) {
        return 0;
    }

    const charges = billingStore.state.productCharges.getPrice();
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
 * Whether the new no-limits UI is enabled.
 */
const noLimitsUiEnabled = computed((): boolean => {
    return configStore.state.config.noLimitsUiEnabled;
});

/**
 * Whether the user is in paid tier.
 */
const isPaidTier = computed((): boolean => {
    return usersStore.state.user.isPaid;
});

/**
 * Whether project members passphrase banner should be shown.
 */
const isTeamPassphraseBanner = computed<boolean>(() => {
    return !usersStore.state.settings.noticeDismissal.projectMembersPassphrase && teamSize.value > 1 && !hasManagedPassphrase.value;
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
 * Returns whether the owner of this project has paid privileges
 */
const ownerHasPaidPrivileges = computed(() => projectsStore.selectedProjectConfig.hasPaidPrivileges);

const hasManagedPassphrase = computed<boolean>(() => projectsStore.selectedProjectConfig.hasManagedPassphrase);

/**
 * Returns whether this project is owned by the current user
 * or whether they're an admin.
 */
const isProjectOwnerOrAdmin = computed(() => {
    const isAdmin = projectsStore.selectedProjectConfig.role === ProjectRole.Admin;
    return isUserProjectOwner.value || isAdmin;
});

/**
 * Returns remaining egress available.
 */
const availableEgress = computed((): number => {
    let diff = (limits.value.userSetBandwidthLimit || limits.value.bandwidthLimit) - limits.value.bandwidthUsed;
    if (ownerHasPaidPrivileges.value && noLimitsUiEnabled.value && !limits.value.userSetBandwidthLimit) {
        diff = Number.MAX_SAFE_INTEGER;
    } else if (!noLimitsUiEnabled.value) {
        diff = limits.value.bandwidthLimit - limits.value.bandwidthUsed;
    }
    return diff < 0 ? 0 : diff;
});

/**
 * Returns percentage of egress limit used.
 */
const egressUsedPercent = computed((): number => {
    return limits.value.bandwidthUsed / (limits.value.userSetBandwidthLimit || limits.value.bandwidthLimit) * 100;
});

/**
 * Returns the CTA text on the bandwidth usage card.
 */
const bandwidthCTA = computed((): string => {
    if (!ownerHasPaidPrivileges.value) {
        return getCTALabel(egressUsedPercent.value);
    }
    if (limits.value.userSetBandwidthLimit) {
        return 'Edit / Remove Limit';
    } else {
        return 'Set Download Limit';
    }
});

/**
 * Returns the used bandwidth text for the storage usage card.
 */
const bandwidthLimitTxt = computed((): string => {
    if (ownerHasPaidPrivileges.value && noLimitsUiEnabled.value && !limits.value.userSetBandwidthLimit) {
        return 'This Month';
    }
    return `Limit: ${usedLimitFormatted(limits.value.userSetBandwidthLimit || limits.value.bandwidthLimit)}`;
});

/**
 * Returns the available bandwidth text for the storage usage card.
 */
const bandwidthAvailableTxt = computed((): string => {
    if (availableEgress.value === Number.MAX_SAFE_INTEGER) {
        return `∞ No Limit`;
    }
    return `${usedLimitFormatted(availableEgress.value)} Available`;
});

/**
 * Returns remaining storage available.
 */
const availableStorage = computed((): number => {
    let diff = (limits.value.userSetStorageLimit || limits.value.storageLimit) - limits.value.storageUsed;
    if (ownerHasPaidPrivileges.value && noLimitsUiEnabled.value && !limits.value.userSetStorageLimit) {
        diff = Number.MAX_SAFE_INTEGER;
    } else if (!noLimitsUiEnabled.value) {
        diff = limits.value.storageLimit - limits.value.storageUsed;
    }
    return diff < 0 ? 0 : diff;
});

/**
 * Returns percentage of storage limit used.
 */
const storageUsedPercent = computed((): number => {
    return limits.value.storageUsed / (limits.value.userSetStorageLimit || limits.value.storageLimit) * 100;
});

/**
 * Returns the CTA text on the storage usage card.
 */
const storageCTA = computed((): string => {
    if (!ownerHasPaidPrivileges.value) {
        return getCTALabel(storageUsedPercent.value);
    }
    if (limits.value.userSetStorageLimit) {
        return 'Edit / Remove Limit';
    } else {
        return 'Set Storage Limit';
    }
});

/**
 * Returns the used storage text for the storage usage card.
 */
const storageLimitTxt = computed((): string => {
    if (ownerHasPaidPrivileges.value && noLimitsUiEnabled.value && !limits.value.userSetStorageLimit) {
        return 'Total';
    }
    return `Limit: ${usedLimitFormatted(limits.value.userSetStorageLimit || limits.value.storageLimit)}`;
});

/**
 * Returns the available storage text for the storage usage card.
 */
const storageAvailableTxt = computed((): string => {
    if (availableStorage.value === Number.MAX_SAFE_INTEGER) {
        return `∞ No Limit`;
    }
    return `${usedLimitFormatted(availableStorage.value)} Available`;
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
    return projectsStore.state.selectedProjectConfig.membersCount;
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
    withTrialCheck(() => { withManagedPassphraseCheck(() => {
        isCreateBucketDialogOpen.value = true;
    });});
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
 * Conditionally opens the upgrade dialog
 * or the edit limit dialog.
 */
function onNeedMoreClicked(source: LimitToChange): void {
    if (isUserProjectOwner.value && !isPaidTier.value && billingEnabled.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }
    if (!ownerHasPaidPrivileges.value) {
        notify.notify('Contact project owner to upgrade to edit limits');
        return;
    }
    if (!isProjectOwnerOrAdmin.value) {
        notify.notify('Contact project owner or admin to edit limits');
        return;
    }
    limitToChange.value = source;
    isEditLimitDialogShown.value = true;
}

/**
 * Returns CTA label based on paid tier status and current usage.
 */
function getCTALabel(usage: number, isSegment = false): string {
    if (isUserProjectOwner.value && !isPaidTier.value && billingEnabled.value) {
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
    if (isUserProjectOwner.value && !isPaidTier.value && billingEnabled.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    window.open('https://storj.dev/support/usage-limit-increases#segment-limit', '_blank', 'noreferrer');
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
    if (isUserProjectOwner.value && !isPaidTier.value && billingEnabled.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }
    if (!ownerHasPaidPrivileges.value) {
        notify.notify('Contact project owner to upgrade to edit limits');
        return;
    }
    if (!isProjectOwnerOrAdmin.value) {
        notify.notify('Contact project owner or admin to edit limits');
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

    const promises: Promise<void | ProjectMembersPage | AccessGrantsPage | AccountBalance | CreditCard[]>[] = [
        agStore.getAccessGrants(FIRST_PAGE, projectID),
        bucketsStore.getBuckets(FIRST_PAGE, projectID),
    ];

    if (emissionImpactViewEnabled.value) {
        promises.push(projectsStore.getEmissionImpact(projectID));
    }

    if (billingEnabled.value) {
        promises.push(
            billingStore.getBalance(),
            billingStore.getCreditCards(),
            billingStore.getCoupon(),
            billingStore.getProductUsageAndChargesCurrentRollup(),
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

onBeforeUnmount((): void => {
    appStore.toggleHasJustLoggedIn(false);
});
</script>
