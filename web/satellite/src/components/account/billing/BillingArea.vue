// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-billing-area">
        <div v-if="isNewBillingScreen" class="account-billing-area__header__div">
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
        <div v-if="!isNewBillingScreen">
            <div v-if="hasNoCreditCard" class="account-billing-area__notification-container">
                <div v-if="isBalanceNegative" class="account-billing-area__notification-container__negative-balance">
                    <NegativeBalanceIcon />
                    <p class="account-billing-area__notification-container__negative-balance__text">
                        Your usage charges exceed your account balance. Please add STORJ Tokens or a debit/credit card to
                        prevent data loss.
                    </p>
                </div>
                <div v-if="isBalanceLow" class="account-billing-area__notification-container__low-balance">
                    <LowBalanceIcon />
                    <p class="account-billing-area__notification-container__low-balance__text">
                        Your account balance is running low. Please add STORJ Tokens or a debit/credit card to prevent data loss.
                    </p>
                </div>
            </div>
            <div v-if="userHasOwnProject" class="account-billing-area__title-area" :class="{ 'custom-position': hasNoCreditCard && (isBalanceLow || isBalanceNegative) }">
                <div class="account-billing-area__title-area__balance-area">
                    <div class="account-billing-area__title-area__balance-area__free-credits">
                        <p class="account-billing-area__title-area__balance-area__free-credits__label">Free Credits:</p>
                        <VLoader v-if="isBalanceFetching" width="20px" height="20px" />
                        <p v-else>{{ balance.freeCredits | centsToDollars }}</p>
                    </div>
                    <div class="account-billing-area__title-area__balance-area__tokens-area" @click.stop="toggleBalanceDropdown">
                        <p class="account-billing-area__title-area__balance-area__tokens-area__label" :style="{ color: balanceColor }">
                            Available Balance:
                        </p>
                        <VLoader v-if="isBalanceFetching" width="20px" height="20px" />
                        <p v-else>
                            {{ balance.coins | centsToDollars }}
                        </p>
                        <HideIcon v-if="isBalanceDropdownShown" class="icon" />
                        <ExpandIcon v-else class="icon" />
                        <HistoryDropdown
                            v-show="isBalanceDropdownShown"
                            label="Balance History"
                            :route="balanceHistoryRoute"
                            @close="closeDropdown"
                        />
                    </div>
                </div>
                <PeriodSelection v-if="userHasOwnProject" />
            </div>
            <EstimatedCostsAndCredits v-if="isSummaryVisible" />
            <PaymentMethods />
            <SmallDepositHistory />
            <CouponArea />
        </div>
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { MetaUtils } from '@/utils/meta';
import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { AccountBalance } from '@/types/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { NavigationLink } from '@/types/navigation';

import PeriodSelection from '@/components/account/billing/depositAndBillingHistory/PeriodSelection.vue';
import SmallDepositHistory from '@/components/account/billing/depositAndBillingHistory/SmallDepositHistory.vue';
import EstimatedCostsAndCredits from '@/components/account/billing/estimatedCostsAndCredits/EstimatedCostsAndCredits.vue';
import CouponArea from '@/components/account/billing/coupons/CouponArea.vue';
import HistoryDropdown from '@/components/account/billing/HistoryDropdown.vue';
import PaymentMethods from '@/components/account/billing/paymentMethods/PaymentMethods.vue';
import VLoader from '@/components/common/VLoader.vue';

import ExpandIcon from '@/../static/images/account/billing/expand.svg';
import HideIcon from '@/../static/images/account/billing/hide.svg';
import LowBalanceIcon from '@/../static/images/account/billing/lowBalance.svg';
import NegativeBalanceIcon from '@/../static/images/account/billing/negativeBalance.svg';

// @vue/component
@Component({
    components: {
        PeriodSelection,
        SmallDepositHistory,
        EstimatedCostsAndCredits,
        PaymentMethods,
        LowBalanceIcon,
        NegativeBalanceIcon,
        HistoryDropdown,
        ExpandIcon,
        HideIcon,
        CouponArea,
        VLoader,
    },
})
export default class BillingArea extends Vue {
    public readonly balanceHistoryRoute: string = RouteConfig.Account.with(RouteConfig.DepositHistory).path;
    public isBalanceFetching = true;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Mounted lifecycle hook after initial render.
     * Fetches account balance.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BALANCE);

            this.isBalanceFetching = false;
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_AREA);
        }
    }

    /**
     * Holds minimum safe balance in cents.
     * If balance is lower - yellow notification should appear.
     */
    private readonly CRITICAL_AMOUNT: number = 1000;

    /**
     * Indicates if free credits dropdown shown.
     */
    public get isCreditsDropdownShown(): boolean {
        return this.$store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.FREE_CREDITS;
    }

    /**
     * Indicates if available balance dropdown shown.
     */
    public get isBalanceDropdownShown(): boolean {
        return this.$store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.AVAILABLE_BALANCE;
    }

    /**
     * Returns account balance from store.
     */
    public get balance(): AccountBalance {
        return this.$store.state.paymentsModule.balance;
    }

    /**
     * Indicates if isEstimatedCostsAndCredits component is visible.
     */
    public get isSummaryVisible(): boolean {
        const isBalancePositive: boolean = this.balance.sum > 0;

        return isBalancePositive || this.userHasOwnProject;
    }

    /**
     * Indicates if no credit cards attached to account.
     */
    public get hasNoCreditCard(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
    }

    /**
     * Indicates if balance is below zero.
     */
    public get isBalanceNegative(): boolean {
        return this.balance.sum < 0;
    }

    /**
     * Indicates if balance is not below zero but lower then CRITICAL_AMOUNT.
     */
    public get isBalanceLow(): boolean {
        return this.balance.coins > 0 && this.balance.sum < this.CRITICAL_AMOUNT;
    }

    /**
     * Returns if balance color red if balance below zero and grey if not.
     */
    public get balanceColor(): string {
        return this.balance.sum < 0 ? '#ff0000' : '#768394';
    }

    /**
     * Indicates if user has own project.
     */
    public get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }

    /**
     * Returns the base account route based on if we're on all projects dashboard.
     */
    public get baseAccountRoute(): NavigationLink {
        if (this.$route.path.includes(RouteConfig.AccountSettings.path)) {
            return RouteConfig.AccountSettings;
        }
        return RouteConfig.Account;
    }

    /**
     * Whether current route name contains term.
     */
    public routeHas(term: string): boolean {
        return !!this.$route.name?.toLowerCase().includes(term);
    }

    /**
     * Toggles available balance dropdown visibility.
     */
    public toggleBalanceDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.AVAILABLE_BALANCE);
    }

    /**
     * Closes free credits and balance dropdowns.
     */
    public closeDropdown(): void {
        if (!this.isCreditsDropdownShown && !this.isBalanceDropdownShown) return;

        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, 'none');
    }

    /**
     * Routes for new billing screens.
     */
    public routeToOverview(): void {
        const overviewPath = this.baseAccountRoute.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path;
        if (this.$route.path !== overviewPath) {
            this.analytics.pageVisit(overviewPath);
            this.$router.push(overviewPath);
        }
    }

    public routeToPaymentMethods(): void {
        const payMethodsPath = this.baseAccountRoute.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).path;
        if (this.$route.path !== payMethodsPath) {
            this.analytics.pageVisit(payMethodsPath);
            this.$router.push(payMethodsPath);
        }
    }

    public routeToBillingHistory(): void {
        const billingPath = this.baseAccountRoute.with(RouteConfig.Billing).with(RouteConfig.BillingHistory2).path;
        if (this.$route.path !== billingPath) {
            this.analytics.pageVisit(billingPath);
            this.$router.push(billingPath);
        }
    }

    public routeToCoupons(): void {
        const couponsPath = this.baseAccountRoute.with(RouteConfig.Billing).with(RouteConfig.BillingCoupons).path;
        if (this.$route.path !== couponsPath) {
            this.analytics.pageVisit(couponsPath);
            this.$router.push(couponsPath);
        }
    }

    /**
     * Indicates if tabs options are hidden.
     */
    public get isNewBillingScreen(): boolean {
        const isNewBillingScreen = MetaUtils.getMetaContent('new-billing-screen');
        return isNewBillingScreen === 'true';
    }

}
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
                font-family: sans-serif;
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
                font-family: sans-serif;
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
