// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-billing-area">
        <div class="account-billing-area__notification-container" v-if="hasNoCreditCard">
            <div class="account-billing-area__notification-container__negative-balance" v-if="isBalanceNegative">
                <NegativeBalanceIcon/>
                <p class="account-billing-area__notification-container__negative-balance__text">
                    Your usage charges exceed your account balance. Please add STORJ Tokens or a debit/credit card to
                    prevent data loss.
                </p>
            </div>
            <div class="account-billing-area__notification-container__low-balance" v-if="isBalanceLow">
                <LowBalanceIcon/>
                <p class="account-billing-area__notification-container__low-balance__text">
                    Your account balance is running low. Please add STORJ Tokens or a debit/credit card to prevent data loss.
                </p>
            </div>
        </div>
        <div class="account-billing-area__title-area" v-if="userHasOwnProject" :class="{ 'custom-position': hasNoCreditCard && (isBalanceLow || isBalanceNegative) }">
            <div class="account-billing-area__title-area__balance-area">
                <div @click.stop="toggleCreditsDropdown" class="account-billing-area__title-area__balance-area__free-credits">
                    <span class="account-billing-area__title-area__balance-area__free-credits__amount">
                        Free Credits: {{ balance.freeCredits | centsToDollars }}
                    </span>
                    <HideIcon v-if="isCreditsDropdownShown"/>
                    <ExpandIcon v-else/>
                    <HistoryDropdown
                        v-show="isCreditsDropdownShown"
                        @close="closeDropdown"
                        label="Credits History"
                        :route="creditHistoryRoute"
                    />
                </div>
                <div @click.stop="toggleBalanceDropdown" class="account-billing-area__title-area__balance-area__tokens-area">
                    <span class="account-billing-area__title-area__balance-area__tokens-area__amount" :style="{ color: balanceColor }">
                        Available Balance: {{ balance.coins | centsToDollars }}
                    </span>
                    <HideIcon v-if="isBalanceDropdownShown"/>
                    <ExpandIcon v-else/>
                    <HistoryDropdown
                        v-show="isBalanceDropdownShown"
                        @close="closeDropdown"
                        label="Balance History"
                        :route="balanceHistoryRoute"
                    />
                </div>
            </div>
            <PeriodSelection v-if="userHasOwnProject"/>
        </div>
        <EstimatedCostsAndCredits v-if="isSummaryVisible"/>
        <PaymentMethods/>
        <SmallDepositHistory/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import PeriodSelection from '@/components/account/billing/depositAndBillingHistory/PeriodSelection.vue';
import SmallDepositHistory from '@/components/account/billing/depositAndBillingHistory/SmallDepositHistory.vue';
import EstimatedCostsAndCredits from '@/components/account/billing/estimatedCostsAndCredits/EstimatedCostsAndCredits.vue';
import HistoryDropdown from '@/components/account/billing/HistoryDropdown.vue';
import PaymentMethods from '@/components/account/billing/paymentMethods/PaymentMethods.vue';

import DatePickerIcon from '@/../static/images/account/billing/datePicker.svg';
import ExpandIcon from '@/../static/images/account/billing/expand.svg';
import HideIcon from '@/../static/images/account/billing/hide.svg';
import LowBalanceIcon from '@/../static/images/account/billing/lowBalance.svg';
import NegativeBalanceIcon from '@/../static/images/account/billing/negativeBalance.svg';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AccountBalance } from '@/types/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        PeriodSelection,
        SmallDepositHistory,
        EstimatedCostsAndCredits,
        PaymentMethods,
        DatePickerIcon,
        LowBalanceIcon,
        NegativeBalanceIcon,
        HistoryDropdown,
        ExpandIcon,
        HideIcon,
    },
})
export default class BillingArea extends Vue {
    public readonly creditHistoryRoute: string = RouteConfig.Account.with(RouteConfig.CreditsHistory).path;
    public readonly balanceHistoryRoute: string = RouteConfig.Account.with(RouteConfig.DepositHistory).path;

    /**
     * Mounted lifecycle hook before initial render.
     * Fetches billing history and project limits.
     */
    public async beforeMount(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PAYMENTS_HISTORY);
            if (this.$store.getters.canUserCreateFirstProject && !this.userHasOwnProject) {
                await this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
            }
        } catch (error) {
            await this.$notify.error(error.message);
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(error.message);
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
        return this.$store.state.appStateModule.appState.isFreeCreditsDropdownShown;
    }

    /**
     * Indicates if available balance dropdown shown.
     */
    public get isBalanceDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isAvailableBalanceDropdownShown;
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
     * Toggles free credits dropdown visibility.
     */
    public toggleCreditsDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_FREE_CREDITS_DROPDOWN);
    }

    /**
     * Toggles available balance dropdown visibility.
     */
    public toggleBalanceDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN);
    }

    /**
     * Closes free credits and balance dropdowns.
     */
    public closeDropdown(): void {
        if (!this.isCreditsDropdownShown && !this.isBalanceDropdownShown) return;

        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }
}
</script>

<style scoped lang="scss">
    .account-billing-area {
        padding-bottom: 40px;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin: 60px 0 20px 0;

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
                    margin-right: 50px;
                    color: #768394;

                    &__amount {
                        margin-right: 10px;
                        font-size: 16px;
                        line-height: 19px;
                    }
                }

                &__free-credits {
                    display: flex;
                    align-items: center;
                    position: relative;
                    cursor: pointer;
                    margin-right: 50px;
                    color: #768394;

                    &__amount {
                        margin-right: 10px;
                        font-size: 16px;
                        line-height: 19px;
                    }
                }
            }
        }

        &__notification-container {
            margin-top: 60px;

            &__negative-balance,
            &__low-balance {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 20px 20px 20px 20px;
                border-radius: 12px;
                margin-bottom: 32px;

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
        margin: 30px 0 20px 0;
    }
</style>
