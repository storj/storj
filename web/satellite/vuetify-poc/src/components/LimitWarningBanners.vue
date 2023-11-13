// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-for="threshold in activeThresholds"
        :key="threshold"
        closable
        variant="tonal"
        :title="bannerText[threshold].title"
        :text="bannerText[threshold].message"
        :type="isHundred(threshold) ? 'error' : 'warning'"
        rounded="lg"
        class="my-2"
        border
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VAlert } from 'vuetify/components';
import { useRouter } from 'vue-router';

import { LimitThreshold, LimitThresholdsReached, LimitType } from '@/types/projects';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { humanizeArray } from '@/utils/strings';
import { Memory } from '@/utils/bytesSize';
import { DEFAULT_PROJECT_LIMITS, useProjectsStore } from '@/store/modules/projectsStore';

type BannerText = {
    title: string;
    message: string;
};

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();

const router = useRouter();

/**
 * Returns which limit thresholds have been reached by which usage limit type.
 */
const reachedThresholds = computed((): LimitThresholdsReached => {
    const reached: LimitThresholdsReached = {
        Eighty: [],
        Hundred: [],
        CustomEighty: [],
        CustomHundred: [],
    };

    const currentLimits = projectsStore.state.currentLimits;
    const config = configStore.state.config;

    if (isAccountFrozen.value || currentLimits === DEFAULT_PROJECT_LIMITS) return reached;

    type LimitInfo = {
        used: number;
        currentLimit: number;
        paidLimit?: number;
    };

    const info: Record<LimitType, LimitInfo> = {
        Storage: {
            used: currentLimits.storageUsed,
            currentLimit: currentLimits.storageLimit,
            paidLimit: parseConfigLimit(config.defaultPaidStorageLimit),
        },
        Egress: {
            used: currentLimits.bandwidthUsed,
            currentLimit: currentLimits.bandwidthLimit,
            paidLimit: parseConfigLimit(config.defaultPaidBandwidthLimit),
        },
        Segment: {
            used: currentLimits.segmentUsed,
            currentLimit: currentLimits.segmentLimit,
        },
    };

    (Object.entries(info) as [LimitType, LimitInfo][]).forEach(([limitType, info]) => {
        const maxLimit = (isPaidTier.value && info.paidLimit) ? Math.max(info.currentLimit, info.paidLimit) : info.currentLimit;
        if (info.used >= maxLimit) {
            reached.Hundred.push(limitType);
        } else if (info.used >= 0.8 * maxLimit) {
            reached.Eighty.push(limitType);
        } else if (isPaidTier.value) {
            if (info.used >= info.currentLimit) {
                reached.CustomHundred.push(limitType);
            } else if (info.used >= 0.8 * info.currentLimit) {
                reached.CustomEighty.push(limitType);
            }
        }
    });

    return reached;
});

/**
 * Indicates if account was frozen due to billing issues.
 */
const isAccountFrozen = computed<boolean>(() => usersStore.state.user.freezeStatus.frozen);

/**
 * Returns the limit thresholds that have been reached by at least 1 usage type.
 */
const activeThresholds = computed<LimitThreshold[]>(() => {
    return (Object.keys(LimitThreshold) as LimitThreshold[]).filter(t => reachedThresholds.value[t].length);
});

/**
 * Returns whether user is in the paid tier.
 */
const isPaidTier = computed<boolean>(() => {
    return usersStore.state.user.paidTier;
});

/**
 * Returns banner title and message.
 */
const bannerText = computed<Record<LimitThreshold, BannerText>>(() => {
    const record = {} as Record<LimitThreshold, BannerText>;

    (Object.keys(LimitThreshold) as LimitThreshold[]).forEach(thresh => {
        let limitText = humanizeArray(reachedThresholds.value[thresh]).toLowerCase() + ' limit';
        if (reachedThresholds.value[thresh].length > 1) limitText += 's';

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
                ? 'You can increase your limits here or in the Project Settings page.'
                : 'Contact support to avoid any service interruptions.';
        }

        record[thresh] = { title, message };
    });

    return record;
});

/**
 * Parses limit value from config, returning it as a byte amount.
 */
function parseConfigLimit(limit: string): number {
    const [value, unit] = limit.split(' ');
    return parseFloat(value) * Memory[unit === 'B' ? 'Bytes' : unit];
}

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
</script>
