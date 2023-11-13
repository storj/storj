// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isLoading" class="loading-overlay active">
        <div class="load" />
        <LoaderImage class="loading-icon" />
    </div>
    <div v-else class="all-dashboard">
        <SessionWrapper>
            <div class="all-dashboard__bars">
                <BetaSatBar v-if="isBetaSatellite" />
                <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="toggleMFARecoveryModal" />
            </div>

            <heading class="all-dashboard__heading" />

            <div class="all-dashboard__content" :class="{ 'no-x-padding': isMyProjectsPage }">
                <div class="all-dashboard__content__divider" />

                <router-view />

                <AllModals />
            </div>
        </SessionWrapper>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { MODALS } from '@/utils/constants/appStatePopUps';
import { User } from '@/types/users';
import {
    AnalyticsErrorEventSource,
} from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { RouteConfig } from '@/types/router';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { CouponType } from '@/types/coupons';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import Heading from '@/views/all-dashboard/components/Heading.vue';
import SessionWrapper from '@/components/utils/SessionWrapper.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import AllModals from '@/components/modals/AllModals.vue';

import LoaderImage from '@/../static/images/common/loadIcon.svg';

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const billingStore = useBillingStore();
const agStore = useAccessGrantsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();

// Minimum number of recovery codes before the recovery code warning bar is shown.
const recoveryCodeWarningThreshold = 4;

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

const isMyProjectsPage = computed((): boolean => {
    return route.path === RouteConfig.AllProjectsDashboard.path;
});

/**
 * Indicates if satellite is in beta.
 */
const isBetaSatellite = computed((): boolean => {
    return configStore.state.config.isBetaSatellite;
});

/**
 * Indicates if loading screen is active.
 */
const isLoading = computed((): boolean => {
    return appStore.state.fetchState === FetchState.LOADING;
});

/**
 * Indicates whether the MFA recovery code warning bar should be shown.
 */
const showMFARecoveryCodeBar = computed((): boolean => {
    const user: User = usersStore.state.user;
    return user.isMFAEnabled && user.mfaRecoveryCodeCount < recoveryCodeWarningThreshold;
});

/**
 * Toggles MFA recovery modal visibility.
 */
function toggleMFARecoveryModal(): void {
    appStore.updateActiveModal(MODALS.mfaRecovery);
}

/**
 * Lifecycle hook after initial render.
 * Pre-fetches user`s and project information.
 */
onMounted(async () => {
    try {
        await Promise.all([
            usersStore.getUser(),
            abTestingStore.fetchValues(),
            usersStore.getSettings(),
        ]);
    } catch (error) {
        if (!(error instanceof ErrorUnauthorized)) {
            appStore.changeState(FetchState.ERROR);
            notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
        }

        setTimeout(async () => await router.push(RouteConfig.Login.path), 1000);

        return;
    }

    try {
        agStore.stopWorker();
        await agStore.startWorker();
    } catch (error) {
        notify.error(`Unable to set access grants wizard. ${error.message}`, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }

    if (billingEnabled.value) {
        try {
            const couponType = await billingStore.setupAccount();
            if (couponType === CouponType.NoCoupon) {
                notify.error(`The coupon code was invalid, and could not be applied to your account`, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
            }

            if (couponType === CouponType.SignupCoupon) {
                notify.success(`The coupon code was added successfully`);
            }
        } catch (error) {
            error.message = `Unable to setup account. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
        }
    }

    try {
        await projectsStore.getUserInvitations();
    } catch (error) {
        error.message = `Unable to get project invitations. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }

    try {
        await projectsStore.getProjects();
    } catch (error) {
        return;
    }

    appStore.changeState(FetchState.LOADED);

    if (usersStore.shouldOnboard && configStore.state.config.pricingPackagesEnabled && !appStore.state.hasShownPricingPlan) {
        appStore.setHasShownPricingPlan(true);
        // if the user is not legible for a pricing plan, they'll automatically be
        // navigated back to all projects dashboard.
        analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.PricingPlanStep).path);
        await router.push(RouteConfig.OnboardingTour.with(RouteConfig.PricingPlanStep).path);
    }
});
</script>

<style scoped lang="scss">
@keyframes rotate {

    from {
        transform: rotate(0deg);
    }

    to {
        transform: rotate(360deg);
    }
}

.no-x-padding {
    padding-left: 0 !important;
    padding-right: 0 !important;
}

.all-dashboard {
    box-sizing: border-box;
    overflow-y: auto;
    width: 100%;
    height: 100%;
    background: var(--c-grey-1);

    &__bars {
        display: contents;
        position: fixed;
        width: 100%;
        top: 0;
        z-index: 1000;
    }

    &__heading {
        margin: 50px auto 0;
        padding: 0 20px;
        max-width: 1200px;
        box-sizing: border-box;

        @media screen and (width <= 500px) {
            margin-top: 0;
            padding: 0;
        }
    }

    &__content {
        padding: 0 20px 50px;
        margin: 0 auto;
        max-width: 1200px;
        box-sizing: border-box;

        &__divider {
            margin: 20px 0;
            border: 0.5px solid var(--c-grey-2);

            @media screen and (width <= 500px) {
                display: none;
            }
        }
    }
}

.load {
    width: 90px;
    height: 90px;
    margin: auto 0;
    border: solid 3px var(--c-blue-3);
    border-radius: 50%;
    border-right-color: transparent;
    border-bottom-color: transparent;
    border-left-color: transparent;
    transition: all 0.5s ease-in;
    animation-name: rotate;
    animation-duration: 1.2s;
    animation-iteration-count: infinite;
    animation-timing-function: linear;
}

.loading-overlay {
    display: flex;
    justify-content: center;
    align-items: center;
    position: absolute;
    inset: 0;
    background-color: var(--c-white);
    visibility: hidden;
    opacity: 0;
    transition: all 0.5s linear;
}

.loading-overlay.active {
    visibility: visible;
    opacity: 1;
}

.loading-icon {
    position: absolute;
    inset: 0;
    margin: auto;
}

:deep(div.account-area-container) {
    padding: 0;
}
</style>
