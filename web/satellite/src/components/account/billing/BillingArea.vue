// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="content" class="account-billing-area">
        <div class="account-billing-area__wrap">
            <div class="account-billing-area__wrap__title">
                <h1 class="account-billing-area__wrap__title__text">Billing</h1>
            </div>
            <v-banner
                v-if="isLowBalance && content"
                class="account-billing-area__wrap__low-balance"
                message="Your STORJ Token balance is low. Deposit more STORJ tokens or add a credit card to avoid interruptions in service."
                link-text="Deposit tokens"
                severity="warning"
                :dashboard-ref="content"
                :on-link-click="onAddTokensClick"
            />
            <div class="account-billing-area__wrap__header">
                <div
                    :class="`account-billing-area__wrap__header__tab first-header-tab ${routeHas('overview') ? 'selected-tab' : ''}`"
                    @click="routeToOverview"
                >
                    <p>Overview</p>
                </div>
                <div
                    :class="`account-billing-area__wrap__header__tab ${routeHas('methods') ? 'selected-tab' : ''}`"
                    @click="routeToPaymentMethods"
                >
                    <p>Payment Methods</p>
                </div>
                <div
                    :class="`account-billing-area__wrap__header__tab ${routeHas('history') ? 'selected-tab' : ''}`"
                    @click="routeToBillingHistory"
                >
                    <p>Billing History</p>
                </div>
                <div
                    :class="`account-billing-area__wrap__header__tab last-header-tab ${routeHas('coupons') ? 'selected-tab' : ''}`"
                    @click="routeToCoupons"
                >
                    <p>Coupons</p>
                </div>
            </div>
            <div class="account-billing-area__wrap__divider" />
        </div>
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { APP_STATE_DROPDOWNS, MODALS } from '@/utils/constants/appStatePopUps';
import { NavigationLink } from '@/types/navigation';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';

import VBanner from '@/components/common/VBanner.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();

const notify = useNotify();
const router = useRouter();
const route = useRoute();
const isLowBalance = useLowTokenBalance();

const content = ref<HTMLElement | null>(null);

/**
 * Indicates if free credits dropdown shown.
 */
const isCreditsDropdownShown = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.FREE_CREDITS;
});

/**
 * Indicates if available balance dropdown shown.
 */
const isBalanceDropdownShown = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.AVAILABLE_BALANCE;
});

/**
 * Returns whether we're on the settings/billing page on the all projects dashboard.
 */
const isOnAllDashboardSettings = computed((): boolean => {
    return route.path.includes(RouteConfig.AccountSettings.path);
});

/**
 * Returns the base account route based on if we're on all projects dashboard.
 */
const baseAccountRoute = computed((): NavigationLink => {
    if (isOnAllDashboardSettings.value) {
        return RouteConfig.AccountSettings;
    }

    return RouteConfig.Account;
});

/**
 * Holds on add tokens button click logic.
 * Triggers Add funds popup.
 */
function onAddTokensClick(): void {
    analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);
    appStore.updateActiveModal(MODALS.addTokenFunds);
}

/**
 * Whether current route name contains term.
 */
function routeHas(term: string): boolean {
    return (route.name as string).toLowerCase().includes(term);
}

/**
 * Closes free credits and balance dropdowns.
 */
function closeDropdown(): void {
    if (!isCreditsDropdownShown.value && !isBalanceDropdownShown.value) return;

    appStore.toggleActiveDropdown('none');
}

/**
 * Routes for new billing screens.
 */
function routeToOverview(): void {
    const overviewPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path;
    if (route.path !== overviewPath) {
        analyticsStore.pageVisit(overviewPath);
        router.push(overviewPath);
    }
}

function routeToPaymentMethods(): void {
    const payMethodsPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).path;
    if (route.path !== payMethodsPath) {
        analyticsStore.pageVisit(payMethodsPath);
        router.push(payMethodsPath);
    }
}

function routeToBillingHistory(): void {
    const billingPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingHistory).path;
    if (route.path !== billingPath) {
        analyticsStore.pageVisit(billingPath);
        router.push(billingPath);
    }
}

function routeToCoupons(): void {
    const couponsPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingCoupons).path;
    if (route.path !== couponsPath) {
        analyticsStore.pageVisit(couponsPath);
        router.push(couponsPath);
    }
}

onMounted(async () => {
    if (!configStore.state.config.nativeTokenPaymentsEnabled) {
        return;
    }

    try {
        await Promise.all([
            billingStore.getBalance(),
            billingStore.getCreditCards(),
            billingStore.getNativePaymentsHistory(),
        ]);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_AREA);
    }
});
</script>

<style scoped lang="scss">
.selected-tab {
    border-bottom: 5px solid black;
}

.account-billing-area {
    padding-bottom: 40px;

    &__wrap {

        &__title {
            padding-top: 20px;

            &__text {
                font-family: 'font_regular', sans-serif;
            }
        }

        &__low-balance {
            margin-top: 25px;
        }

        &__header {
            width: 100%;
            max-width: 750px;
            height: 40px;
            display: flex;
            align-content: center;
            justify-content: space-between;
            padding-top: 25px;
            overflow-y: auto;

            /* Hide scrollbar for IE, Edge and Firefox */
            -ms-overflow-style: none;  /* IE and Edge */
            scrollbar-width: none;  /* Firefox */

            /* Hide scrollbar for Chrome, Safari and Opera */

            &::-webkit-scrollbar {
                display: none;
            }

            &__tab {
                font-family: 'font_regular', sans-serif;
                color: var(--c-grey-6);
                font-size: 16px;
                height: auto;
                width: auto;
                transition-duration: 50ms;
                white-space: nowrap;
                padding: 0 8px;
            }

            &__tab:hover {
                border-bottom: 5px solid black;
                cursor: pointer;
            }
        }

        &__divider {
            width: 100%;
            border-bottom: 1px solid #dadfe7;
        }
    }
}

@media only screen and (width <= 625px) {

    .account-billing-area {

        &__wrap {
            margin-right: -24px;
            margin-left: -24px;

            &__title {
                margin-left: 24px;
            }

            &__low-balance {
                margin: 25px 24px 0;
            }
        }
    }

    .first-header-tab {
        margin-left: 24px;
    }

    .last-header-tab {
        margin-right: 24px;
    }
}
</style>
