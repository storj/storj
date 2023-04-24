// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-billing-area">
        <div class="account-billing-area__header__div">
            <div class="account-billing-area__title">
                <h1 class="account-billing-area__title__text">Billing</h1>
            </div>
            <div class="account-billing-area__header">
                <div
                    :class="`account-billing-area__header__tab first-header-tab ${routeHas('overview') ? 'selected-tab' : ''}`"
                    @click="routeToOverview"
                >
                    <p>Overview</p>
                </div>
                <div
                    :class="`account-billing-area__header__tab ${routeHas('methods') ? 'selected-tab' : ''}`"
                    @click="routeToPaymentMethods"
                >
                    <p>Payment Methods</p>
                </div>
                <div
                    :class="`account-billing-area__header__tab ${routeHas('history') ? 'selected-tab' : ''}`"
                    @click="routeToBillingHistory"
                >
                    <p>Billing History</p>
                </div>
                <div
                    :class="`account-billing-area__header__tab last-header-tab ${routeHas('coupons') ? 'selected-tab' : ''}`"
                    @click="routeToCoupons"
                >
                    <p>Coupons</p>
                </div>
            </div>
            <div class="account-billing-area__divider" />
        </div>
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive } from 'vue';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { NavigationLink } from '@/types/navigation';
import { useNotify, useRouter } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';

const appStore = useAppStore();
const billingStore = useBillingStore();
const notify = useNotify();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

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
 * Returns the base account route based on if we're on all projects dashboard.
 */
const baseAccountRoute = computed((): NavigationLink => {
    if (router.currentRoute.path.includes(RouteConfig.AccountSettings.path)) {
        return RouteConfig.AccountSettings;
    }

    return RouteConfig.Account;
});

/**
 * Whether current route name contains term.
 */
function routeHas(term: string): boolean {
    return !!router.currentRoute.name?.toLowerCase().includes(term);
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
    if (router.currentRoute.path !== overviewPath) {
        analytics.pageVisit(overviewPath);
        router.push(overviewPath);
    }
}

function routeToPaymentMethods(): void {
    const payMethodsPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).path;
    if (router.currentRoute.path !== payMethodsPath) {
        analytics.pageVisit(payMethodsPath);
        router.push(payMethodsPath);
    }
}

function routeToBillingHistory(): void {
    const billingPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingHistory).path;
    if (router.currentRoute.path !== billingPath) {
        analytics.pageVisit(billingPath);
        router.push(billingPath);
    }
}

function routeToCoupons(): void {
    const couponsPath = baseAccountRoute.value.with(RouteConfig.Billing).with(RouteConfig.BillingCoupons).path;
    if (router.currentRoute.path !== couponsPath) {
        analytics.pageVisit(couponsPath);
        router.push(couponsPath);
    }
}

/**
 * Mounted lifecycle hook after initial render.
 * Fetches account balance.
 */
onMounted(async (): Promise<void> => {
    try {
        await billingStore.getBalance();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_AREA);
    }
});
</script>

<style scoped lang="scss">
    .label-header {
        display: none;
    }

    .credit-history {

        &__coupon-modal-wrapper {
            background: #1b2533c7 75%;
            position: fixed;
            width: 100%;
            height: 100%;
            top: 0;
            left: 0;
            z-index: 1000;
        }

        &__coupon-modal {
            width: 741px;
            height: 298px;
            background: #fff;
            border-radius: 8px;
            margin: 15% auto;
            position: relative;

            &__header-wrapper {
                display: flex;
                justify-content: space-between;
            }

            &__header {
                font-family: 'font_bold', sans-serif;
                font-style: normal;
                font-weight: bold;
                font-size: 16px;
                line-height: 148.31%;
                margin: 30px 0 10px;
                display: inline-block;
            }

            &__input-wrapper {
                position: relative;
                width: 85%;
                margin: 0 auto;

                .headerless-input::placeholder {
                    color: #384b65;
                    opacity: 0.4;
                    position: relative;
                    left: 20px;
                }
            }

            &__claim-button {
                position: absolute;
                bottom: 11px;
                right: 10px;
            }

            &__apply-button {
                width: 85%;
                height: 44px;
                position: absolute;
                left: 0;
                right: 0;
                margin: 0 auto;
                bottom: 50px;
                background: #93a1af;
            }

            &__icon {
                position: absolute;
                top: 90px;
                z-index: 1;
                left: 20px;
            }
        }
    }

    .selected-tab {
        border-bottom: 5px solid black;
    }

    .account-billing-area {
        padding-bottom: 40px;

        &__title {
            padding-top: 20px;

            &__text {
                font-family: 'font_regular', sans-serif;
            }
        }

        &__divider {
            width: 100%;
            border-bottom: 1px solid #dadfe7;
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

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin: 20px 0;

            &__balance-area {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-family: 'font_regular', sans-serif;

                &__tokens-area {
                    display: flex;
                    align-items: center;
                    position: relative;
                    cursor: pointer;
                    color: #768394;
                    font-size: 16px;
                    line-height: 19px;

                    &__label {
                        margin-right: 10px;
                        white-space: nowrap;
                    }
                }

                &__free-credits {
                    display: flex;
                    align-items: center;
                    position: relative;
                    cursor: default;
                    margin-right: 50px;
                    color: #768394;
                    font-size: 16px;
                    line-height: 19px;

                    &__label {
                        margin-right: 10px;
                        white-space: nowrap;
                    }
                }
            }
        }

        &__notification-container {
            margin-top: 20px;

            &__negative-balance,
            &__low-balance {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 20px;
                border-radius: 12px;

                &__text {
                    font-family: 'font_medium', sans-serif;
                    margin: 0 17px;
                    font-size: 14px;
                    font-weight: 500;
                    line-height: 19px;
                }
            }

            &__negative-balance {
                background-color: #ffd4d2;
            }

            &__low-balance {
                background-color: #fcf8e3;
            }
        }
    }

    .custom-position {
        margin: 30px 0 20px;
    }

    .icon {
        min-width: 14px;
        margin-left: 10px;
    }

    @media only screen and (max-width: 625px) {

        .account-billing-area__header__div {
            margin-right: -24px;
            margin-left: -24px;
        }

        .account-billing-area__title {
            margin-left: 24px;
        }

        .first-header-tab {
            margin-left: 24px;
        }

        .last-header-tab {
            margin-right: 24px;
        }
    }
</style>
