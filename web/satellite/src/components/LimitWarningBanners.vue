// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-for="threshold in activeThresholds"
        :key="threshold"
        closable
        variant="tonal"
        :title="bannerText[threshold].title"
        :type="bannerText[threshold].hundred ? 'error' : 'warning'"
        class="my-2"
        border
    >
        <template v-if="!isProjectOwner" #text>
            Contact project owner to upgrade to avoid any service interruptions.
        </template>
        <template v-else-if="hasPaidPrivileges && !reachedThresholds[threshold].includes(LimitType.Segment)" #text>
            You can increase your limits
            <a class="text-decoration-underline text-cursor-pointer" @click="openLimitDialog(bannerText[threshold].limitType)">here</a>
            or in the
            <a class="text-decoration-underline text-cursor-pointer" @click="goToProjectSettings">project settings page</a>.
        </template>
        <template v-else-if="!hasPaidPrivileges && bannerText[threshold].hundred" #text>
            <a class="text-decoration-underline text-cursor-pointer" @click="appStore.toggleUpgradeFlow(true)">Upgrade</a> to avoid any service interruptions.
        </template>
        <template v-else-if="!hasPaidPrivileges && !bannerText[threshold].hundred" #text>
            Avoid interrupting your usage by
            <a class="text-decoration-underline text-cursor-pointer" @click="appStore.toggleUpgradeFlow(true)">upgrading</a>
            your account.
        </template>
        <template v-else #text>
            Contact support to avoid any service interruptions.
        </template>
    </v-alert>

    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="LimitToChange.Storage" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VAlert } from 'vuetify/components';
import { useRouter } from 'vue-router';

import { LimitThreshold, LimitThresholdsReached, LimitToChange, LimitType } from '@/types/projects';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { humanizeArray } from '@/utils/strings';
import { Memory } from '@/utils/bytesSize';
import { DEFAULT_PROJECT_LIMITS, useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@/store/modules/appStore';
import { ROUTES } from '@/router';

import EditProjectLimitDialog from '@/components/dialogs/EditProjectLimitDialog.vue';

type BannerText = {
    title: string;
    hundred: boolean;
    limitType: LimitType;
};

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const router = useRouter();

const isEditLimitDialogShown = ref(false);
const limitToChange = ref(LimitToChange.Storage);

/**
 * Returns whether this project is owner by the current user
 */
const isProjectOwner = computed(() => {
    return projectsStore.state.selectedProject.ownerId === usersStore.state.user.id;
});

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
        const maxLimit = (hasPaidPrivileges.value && info.paidLimit) ? Math.max(info.currentLimit, info.paidLimit) : info.currentLimit;
        if (info.used >= maxLimit) {
            reached.Hundred.push(limitType);
        } else if (info.used >= 0.8 * maxLimit) {
            reached.Eighty.push(limitType);
        } else if (hasPaidPrivileges.value) {
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
 * Returns whether user has paid privileges.
 */
const hasPaidPrivileges = computed<boolean>(() => {
    return usersStore.state.user.hasPaidPrivileges;
});

/**
 * Returns banner title and message.
 */
const bannerText = computed<Record<LimitThreshold, BannerText>>(() => {
    const record = {} as Record<LimitThreshold, BannerText>;

    (Object.keys(LimitThreshold) as LimitThreshold[]).forEach(thresh => {
        let limitText = humanizeArray(reachedThresholds.value[thresh]).toLowerCase() + ' limit';
        if (reachedThresholds.value[thresh].length > 1) limitText += 's';

        const hundred = isHundred(thresh);

        const title = hundred
            ? `URGENT: You've reached the ${limitText} for your project.`
            : `You've used 80% of your ${limitText}.`;

        record[thresh] = { title, hundred, limitType: reachedThresholds.value[thresh][0] };
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
 * Opens the limit dialog.
 */
function openLimitDialog(limitType: LimitType): void {
    switch (limitType) {
    case LimitType.Storage:
        limitToChange.value = LimitToChange.Storage;
        break;
    case LimitType.Egress:
        limitToChange.value = LimitToChange.Bandwidth;
        break;
    default:
        return;
    }
    isEditLimitDialogShown.value = true;
}

/**
 * Navigates to the project settings page.
 */
function goToProjectSettings(): void {
    router.push({
        name: ROUTES.ProjectSettings.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
}
</script>
