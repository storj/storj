// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="all-dashboard-banners">
        <UpgradeNotification
            v-if="isPaidTierBannerShown"
            class="all-dashboard-banners__upgrade"
            :open-add-p-m-modal="togglePMModal"
        />

        <v-banner
            v-if="isAccountFrozen && parentRef"
            class="all-dashboard-banners__freeze"
            severity="critical"
            :dashboard-ref="parentRef"
        >
            <template #text>
                <p class="medium">Your account was frozen due to billing issues. Please update your payment information.</p>
                <p class="link" @click.stop.self="redirectToBillingPage">To Billing Page</p>
            </template>
        </v-banner>

        <v-banner
            v-if="isAccountWarned && parentRef"
            class="all-dashboard-banners__warning"
            severity="warning"
            :dashboard-ref="parentRef"
        >
            <template #text>
                <p class="medium">Your account will be frozen soon due to billing issues. Please update your payment information.</p>
                <p class="link" @click.stop.self="redirectToBillingPage">To Billing Page</p>
            </template>
        </v-banner>

        <v-banner
            v-if="limitState.hundredIsShown && parentRef"
            class="all-dashboard-banners__hundred-limit"
            severity="critical"
            :on-click="() => setIsHundredLimitModalShown(true)"
            :dashboard-ref="parentRef"
        >
            <template #text>
                <p class="medium">{{ limitState.hundredLabel }}</p>
                <p class="link" @click.stop.self="togglePMModal">Upgrade now</p>
            </template>
        </v-banner>

        <v-banner
            v-if="limitState.eightyIsShown && parentRef"
            class="all-dashboard-banners__eighty-limit"
            severity="warning"
            :on-click="() => setIsEightyLimitModalShown(true)"
            :dashboard-ref="parentRef"
        >
            <template #text>
                <p class="medium">{{ limitState.eightyLabel }}</p>
                <p class="link" @click.stop.self="togglePMModal">Upgrade now</p>
            </template>
        </v-banner>
    </div>
</template>

<script setup lang="ts">

import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';

import { useUsersStore } from '@/store/modules/usersStore';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { RouteConfig } from '@/types/router';

import VBanner from '@/components/common/VBanner.vue';
import UpgradeNotification from '@/components/notifications/UpgradeNotification.vue';

const router = useRouter();

const usersStore = useUsersStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();

const props = defineProps<{
  parentRef: HTMLElement;
}>();

const isHundredLimitModalShown = ref<boolean>(false);
const isEightyLimitModalShown = ref<boolean>(false);

/**
 * Returns all needed information for limit banner and modal when bandwidth or storage close to limits.
 */
type LimitedState = {
  eightyIsShown: boolean;
  hundredIsShown: boolean;
  eightyLabel: string;
  eightyModalLimitType: string;
  eightyModalTitle: string;
  hundredLabel: string;
  hundredModalTitle: string;
  hundredModalLimitType: string;
}
const limitState = computed((): LimitedState => {
    const result: LimitedState = {
        eightyIsShown: false,
        hundredIsShown: false,
        eightyLabel: '',
        eightyModalLimitType: '',
        eightyModalTitle: '',
        hundredLabel: '',
        hundredModalTitle: '',
        hundredModalLimitType: '',
    };

    if (usersStore.state.user.paidTier || isAccountFrozen.value) {
        return result;
    }

    const currentLimits = projectsStore.state.currentLimits;

    const limitTypeArr = [
        { name: 'egress', usedPercent: Math.round(currentLimits.bandwidthUsed * 100 / currentLimits.bandwidthLimit) },
        { name: 'storage', usedPercent: Math.round(currentLimits.storageUsed * 100 / currentLimits.storageLimit) },
        { name: 'segment', usedPercent: Math.round(currentLimits.segmentUsed * 100 / currentLimits.segmentLimit) },
    ];

    const hundredPercent: string[] = [];
    const eightyPercent: string[] = [];

    limitTypeArr.forEach((limitType) => {
        if (limitType.usedPercent >= 80) {
            if (limitType.usedPercent >= 100) {
                hundredPercent.push(limitType.name);
            } else {
                eightyPercent.push(limitType.name);
            }
        }
    });

    if (eightyPercent.length !== 0) {
        result.eightyIsShown = true;

        const eightyPercentString = eightyPercent.join(' and ');

        result.eightyLabel = `You've used 80% of your ${eightyPercentString} limit. Avoid interrupting your usage by upgrading your account.`;
        result.eightyModalTitle = `80% ${eightyPercentString} limit used`;
        result.eightyModalLimitType = eightyPercentString;
    }

    if (hundredPercent.length !== 0) {
        result.hundredIsShown = true;

        const hundredPercentString = hundredPercent.join(' and ');

        result.hundredLabel = `URGENT: You’ve reached the ${hundredPercentString} limit for your project. Upgrade to avoid any service interruptions.`;
        result.hundredModalTitle = `URGENT: You’ve reached the ${hundredPercentString} limit for your project.`;
        result.hundredModalLimitType = hundredPercentString;
    }

    return result;
});

/**
 * Indicates if account was frozen due to billing issues.
 */
const isAccountFrozen = computed((): boolean => {
    return usersStore.state.user.freezeStatus.frozen;
});

/**
 * Indicates if account was warned due to billing issues.
 */
const isAccountWarned = computed((): boolean => {
    return usersStore.state.user.freezeStatus.warned;
});

/* whether the paid tier banner should be shown */
const isPaidTierBannerShown = computed((): boolean => {
    return !usersStore.state.user.paidTier
      && joinedWhileAgo.value;
});

/* whether the user joined more than 7 days ago */
const joinedWhileAgo = computed((): boolean => {
    const createdAt = usersStore.state.user.createdAt as Date | null;
    if (!createdAt) return true; // true so we can show the banner regardless
    const millisPerDay = 24 * 60 * 60 * 1000;
    return ((Date.now() - createdAt.getTime()) / millisPerDay) > 7;
});

/**
 * Opens add payment method modal.
 */
function togglePMModal(): void {
    isHundredLimitModalShown.value = false;
    isEightyLimitModalShown.value = false;

    if (!usersStore.state.user.paidTier) {
        appStore.updateActiveModal(MODALS.upgradeAccount);
    }
}

function setIsEightyLimitModalShown(value: boolean): void {
    isEightyLimitModalShown.value = value;
}

function setIsHundredLimitModalShown(value: boolean): void {
    isHundredLimitModalShown.value = value;
}

/**
 * Redirects to Billing Page.
 */
async function redirectToBillingPage(): Promise<void> {
    await router.push(RouteConfig.AccountSettings.with(RouteConfig.Billing2.with(RouteConfig.BillingPaymentMethods2)).path);
}
</script>

<style scoped lang="scss">
.all-dashboard-banners {
    margin-bottom: 20px;

    &__upgrade,
    &__project-limit,
    &__freeze,
    &__warning,
    &__hundred-limit,
    &__eighty-limit {
        margin: 20px 0 0;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
    }
}
</style>
