// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="limits-area">
        <LimitCard
            :icon="StorageIcon"
            title="Storage"
            color="#537cff"
            :used-value="storageUsed"
            :used-title="`${usedOrLimitFormatted(limits.storageUsed)} Used`"
            :used-info="`Storage limit: ${usedOrLimitFormatted(limits.storageLimit, true)}`"
            :available-title="`${availableFormatted(limits.storageLimit - limits.storageUsed)} Available`"
            :action-title="usageActionTitle(storageUsed)"
            :on-action="() => usageAction(LimitToChange.Storage)"
            :is-loading="isLoading"
            use-action
        />
        <LimitCard
            :icon="DownloadIcon"
            title="Download"
            color="#7b61ff"
            :used-value="bandwidthUsed"
            :used-title="`${usedOrLimitFormatted(limits.bandwidthUsed)} Used`"
            :used-info="`Download limit: ${usedOrLimitFormatted(limits.bandwidthLimit, true)} per month`"
            :available-title="`${availableFormatted(limits.bandwidthLimit - limits.bandwidthUsed)} Available`"
            :action-title="usageActionTitle(bandwidthUsed)"
            :on-action="() => usageAction(LimitToChange.Bandwidth)"
            :is-loading="isLoading"
            use-action
        />
        <LimitCard
            :icon="SegmentIcon"
            title="Segments"
            color="#003dc1"
            :used-value="segmentUsed"
            :used-title="`${limits.segmentUsed.toLocaleString()} Used`"
            :used-info="`Segment limit: ${limits.segmentLimit.toLocaleString()}`"
            :available-title="`${segmentsAvailable.toLocaleString()} Available`"
            :action-title="usageActionTitle(segmentUsed, true)"
            :on-action="startUpgradeFlow"
            :is-loading="isLoading"
            :use-action="!isPaidTier"
            :link="segmentUsed < EIGHTY_PERCENT ?
                'https://docs.storj.io/dcs/pricing#per-segment-fee' :
                'https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212'"
        />
        <LimitCard
            v-if="coupon && isFreeTierCoupon"
            :icon="CheckmarkIcon"
            title="Free Tier"
            color="#091c45"
            :used-value="projectPricePercentage"
            :used-title="`${projectPricePercentage.toFixed(0)}% Used`"
            :used-info="`Free tier: ${centsToDollars(coupon.amountOff)}`"
            :available-title="`${remainingCouponAmount}% Available`"
            :action-title="freeTierActionTitle"
            :on-action="startUpgradeFlow"
            :is-loading="isLoading"
            :is-dark="isPaidTier"
            :use-action="!isPaidTier"
            link="https://docs.storj.io/dcs/pricing#free-tier"
        />
        <LimitCard
            v-if="coupon && !isFreeTierCoupon"
            :icon="CheckmarkIcon"
            title="Coupon"
            color="#091c45"
            :used-value="projectPricePercentage"
            :used-title="`${projectPricePercentage.toFixed(0)}% Used`"
            :used-info="`Coupon: ${centsToDollars(coupon.amountOff)} monthly`"
            :available-title="isPaidTier ?
                `${centsToDollars(coupon.amountOff)} per month` :
                `${remainingCouponAmount}% Available`"
            action-title="View coupons"
            :on-action="navigateToCoupons"
            :is-loading="isLoading"
            is-dark
            use-action
        />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { LimitToChange, ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { Size } from '@/utils/bytesSize';
import { Coupon, ProjectCharges } from '@/types/payments';
import { centsToDollars } from '@/utils/strings';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { RouteConfig } from '@/types/router';

import LimitCard from '@/components/project/dashboard/LimitCard.vue';

import StorageIcon from '@/../static/images/project/cloud.svg';
import DownloadIcon from '@/../static/images/project/download.svg';
import SegmentIcon from '@/../static/images/project/segment.svg';
import CheckmarkIcon from '@/../static/images/project/checkmark.svg';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const billingStore = useBillingStore();
const router = useRouter();

const props = defineProps<{
    isLoading: boolean
}>();

const EIGHTY_PERCENT = 80;
const HUNDRED_PERCENT = 100;

/**
 * Returns coupon from store.
 */
const coupon = computed((): Coupon | null => {
    return billingStore.state.coupon;
});

/**
 * Indicates if active coupon is free tier coupon.
 */
const isFreeTierCoupon = computed((): boolean => {
    if (!coupon.value) {
        return true;
    }

    const freeTierCouponName = 'Free Tier';

    return coupon.value.name.includes(freeTierCouponName);
});

/**
 * Indicates if user is in a paid tier status.
 */
const isPaidTier = computed((): boolean => {
    return usersStore.state.user.paidTier;
});

/**
 * Indicates if user is project owner.
 */
const isProjectOwner = computed((): boolean => {
    return projectsStore.state.selectedProject.ownerId === usersStore.state.user.id;
});

/**
 * Returns current project limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Returns current project charges from store.
 */
const projectCharges = computed((): ProjectCharges => {
    return billingStore.state.projectCharges as ProjectCharges;
});

/**
 * Calculates storage usage percentage.
 */
const storageUsed = computed((): number => {
    return (limits.value.storageUsed / limits.value.storageLimit) * HUNDRED_PERCENT;
});

/**
 * Calculates bandwidth usage percentage.
 */
const bandwidthUsed = computed((): number => {
    return (limits.value.bandwidthUsed / limits.value.bandwidthLimit) * HUNDRED_PERCENT;
});

/**
 * Calculates segment usage percentage.
 */
const segmentUsed = computed((): number => {
    return (limits.value.segmentUsed / limits.value.segmentLimit) * HUNDRED_PERCENT;
});

/**
 * Calculates overall project price percentage depending on current coupon for current month.
 */
const projectPricePercentage = computed((): number => {
    if (!coupon.value) {
        return 0;
    }

    const selectedProjectID = projectsStore.state.selectedProject.id;

    let projectPrice = projectCharges.value.getProjectPrice(selectedProjectID);
    if (projectPrice > coupon.value.amountOff) {
        projectPrice = coupon.value.amountOff;
    }

    return (projectPrice / coupon.value.amountOff) * HUNDRED_PERCENT;
});

/**
 * Calculates remaining coupon amount percentage.
 */
const remainingCouponAmount = computed((): string => {
    return (HUNDRED_PERCENT - projectPricePercentage.value).toFixed(0);
});

/**
 * Returns free tier card CTA label.
 */
const freeTierActionTitle = computed((): string => {
    switch (true) {
    case !isPaidTier.value && projectPricePercentage.value >= EIGHTY_PERCENT && projectPricePercentage.value < HUNDRED_PERCENT:
        return 'Upgrade';
    case !isPaidTier.value && projectPricePercentage.value >= HUNDRED_PERCENT:
        return 'Upgrade now';
    default:
        return 'Learn more';
    }
});

/**
 * Calculates remaining available segments amount.
 */
const segmentsAvailable = computed((): number => {
    let available = limits.value.segmentLimit - limits.value.segmentUsed;
    if (available < 0) {
        available = 0;
    }

    return available;
});

/**
 * Returns usage card CTA label.
 */
function usageActionTitle(usage: number, isSegment = false): string {
    switch (true) {
    case !isProjectOwner.value:
        return '';
    case !isPaidTier.value && usage < EIGHTY_PERCENT:
        return 'Need more?';
    case !isPaidTier.value && usage >= EIGHTY_PERCENT && usage < HUNDRED_PERCENT:
        return 'Upgrade';
    case !isPaidTier.value && usage >= HUNDRED_PERCENT:
        return 'Upgrade now';
    case isPaidTier.value && usage < EIGHTY_PERCENT && !isSegment:
        return 'Change limits';
    case isPaidTier.value && usage < EIGHTY_PERCENT && isSegment:
        return 'Learn more';
    case isPaidTier.value && usage >= EIGHTY_PERCENT:
        return 'Increase limits';
    default:
        return '';
    }
}

/**
 * Returns formatted value of available usage.
 */
function availableFormatted(diff: number): string {
    const size = new Size(diff);

    let value = size.formattedBytes;
    if (parseFloat(value) < 0) {
        value = '0';
    }

    return `${value} ${size.label}`;
}

/**
 * Returns formatted value of used amount.
 */
function usedOrLimitFormatted(value: number, withoutSpace = false): string {
    const size = new Size(value);

    let formatted = `${size.formattedBytes} ${size.label}`;
    if (withoutSpace) {
        formatted = formatted.replace(' ', '');
    }

    return formatted;
}

/**
 * Handles usage card CTA click.
 */
function usageAction(limit: LimitToChange): void {
    if (!isPaidTier.value) {
        startUpgradeFlow();
        return;
    }

    appStore.setActiveChangeLimit(limit);
    appStore.updateActiveModal(MODALS.changeProjectLimit);
}

/**
 * Starts upgrade account flow.
 */
function startUpgradeFlow(): void {
    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Navigates to billing coupons view.
 */
function navigateToCoupons(): void {
    router.push(RouteConfig.Account.with(RouteConfig.Billing.with(RouteConfig.BillingCoupons)).path);
}
</script>

<style scoped lang="scss">
.limits-area {
    display: grid;
    grid-template-columns: calc(50% - 8px) calc(50% - 8px);
    grid-gap: 16px;
    margin-top: 16px;

    @media screen and (width <= 750px) {
        grid-template-columns: auto;
    }
}
</style>
