// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-banner
        v-for="threshold in activeThresholds"
        :key="threshold"
        :title="bannerText[threshold].value.title"
        :message="bannerText[threshold].value.message"
        :severity="isHundred(threshold) ? 'critical' : 'warning'"
        :link-text="!isPaidTier ? 'Upgrade Now' : isCustom(threshold) ? 'Edit Limits' : 'Contact Support'"
        :href="(isPaidTier && !isCustom(threshold)) ? requestURL : undefined"
        :dashboard-ref="dashboardRef"
        :on-click="() => onBannerClick(threshold)"
        :on-link-click="() => onLinkClick(threshold)"
    />
</template>

<script setup lang="ts">
import { computed, ComputedRef } from 'vue';
import { useRouter } from 'vue-router';

import { LimitThreshold, LimitThresholdsReached } from '@/types/projects';
import { humanizeArray } from '@/utils/strings';
import { useUsersStore } from '@/store/modules/usersStore';
import { RouteConfig } from '@/types/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VBanner from '@/components/common/VBanner.vue';

const usersStore = useUsersStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();

const router = useRouter();

const props = defineProps<{
    reachedThresholds: LimitThresholdsReached;
    onBannerClick: (threshold: LimitThreshold) => void;
    onUpgradeClick: () => void;
    dashboardRef: HTMLElement;
}>();

/**
 * Returns the limit thresholds that have been reached by at least 1 usage type.
 */
const activeThresholds = computed<LimitThreshold[]>(() => {
    return (Object.keys(LimitThreshold) as LimitThreshold[]).filter(t => props.reachedThresholds[t].length);
});

/**
 * Returns whether user is in the paid tier.
 */
const isPaidTier = computed<boolean>(() => {
    return usersStore.state.user.paidTier;
});

type BannerText = {
    title: string;
    message: string;
};

const bannerText = {} as Record<LimitThreshold, ComputedRef<BannerText>>;
(Object.keys(LimitThreshold) as LimitThreshold[]).forEach(thresh => {
    bannerText[thresh] = computed<BannerText>(() => {
        let limitText = humanizeArray(props.reachedThresholds[thresh]).toLowerCase() + ' limit';
        if (props.reachedThresholds[thresh].length > 1) limitText += 's';

        const custom = isCustom(thresh);
        const hundred = isHundred(thresh);

        const title = hundred
            ? `URGENT: You've reached the ${limitText} for your project.`
            : `You've used 80% of your ${limitText}.`;

        let message: string;
        if (!isPaidTier.value) {
            message = hundred
                ? 'Upgrade to avoid any service interruptions.'
                : 'Avoid interrupting your usage by upgrading your account.';
        } else {
            message = custom
                ? 'You can increase your limits in the Project Settings page.'
                : 'Contact support to avoid any service interruptions.';
        }

        return { title, message };
    });
});

/**
 * Returns the URL for the general request page from the store.
 */
const requestURL = computed<string>(() => {
    return configStore.state.config.generalRequestURL;
});

/**
 * Returns whether the threshold represents 100% usage.
 */
function isHundred(threshold: LimitThreshold): boolean {
    return threshold.toLowerCase().includes('hundred');
}

/**
 * Returns whether the threshold is for a custom limit.
 */
function isCustom(threshold: LimitThreshold): boolean {
    return threshold.toLowerCase().includes('custom');
}

/**
 * Handles click event for link appended to banner.
 */
function onLinkClick(threshold: LimitThreshold): void {
    if (!isPaidTier.value) {
        props.onUpgradeClick();
        return;
    }
    if (isCustom(threshold)) {
        analyticsStore.pageVisit(RouteConfig.EditProjectDetails.path);
        router.push(RouteConfig.EditProjectDetails.path);
        return;
    }
}
</script>
