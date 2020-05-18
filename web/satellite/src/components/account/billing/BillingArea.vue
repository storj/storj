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
                <span class="account-billing-area__title-area__balance-area__free-credits">
                    Free Credits: {{ balance.freeCredits | centsToDollars }}
                </span>
                <span class="account-billing-area__title-area__balance-area__tokens" :style="{ color: balanceColor }">
                    STORJ Balance: {{ balance.coins | centsToDollars }}
                </span>
            </div>
            <div class="account-billing-area__title-area__options-area" v-if="userHasOwnProject">
                <div class="account-billing-area__title-area__options-area__option active" @click.prevent="onCurrentPeriodClick">
                    <span class="account-billing-area__title-area__options-area__option__label">Current Billing Period</span>
                </div>
                <div class="account-billing-area__title-area__options-area__option" @click.prevent="onPreviousPeriodClick">
                    <span class="account-billing-area__title-area__options-area__option__label">Previous Billing Period</span>
                </div>
                <div class="account-billing-area__title-area__options-area__option datepicker" @click.prevent.self="onCustomDateClick">
                    <VDatepicker
                        ref="datePicker"
                        :date="startTime"
                        @change="getDates"
                    />
                    <DatePickerIcon
                        class="account-billing-area__title-area__options-area__option__image"
                        @click.prevent="onCustomDateClick"
                    />
                </div>
            </div>
        </div>
        <EstimatedCostsAndCredits v-if="isSummaryVisible"/>
        <PaymentMethods/>
        <DepositAndBilling/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DepositAndBilling from '@/components/account/billing/billingHistory/DepositAndBilling.vue';
import EstimatedCostsAndCredits from '@/components/account/billing/estimatedCostsAndCredits/EstimatedCostsAndCredits.vue';
import PaymentMethods from '@/components/account/billing/paymentMethods/PaymentMethods.vue';
import VDatepicker from '@/components/common/VDatePicker.vue';

import DatePickerIcon from '@/../static/images/account/billing/datePicker.svg';
import LowBalanceIcon from '@/../static/images/account/billing/lowBalance.svg';
import NegativeBalanceIcon from '@/../static/images/account/billing/negativeBalance.svg';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AccountBalance, DateRange } from '@/types/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { ProjectOwning } from '@/utils/projectOwning';

/**
 * Exposes empty time for DatePicker.
 */
class StartTime {
    public time = null;
}

/**
 * Exposes VDatepicker's showCheck method.
 */
declare interface ShowCheck {
    showCheck(): void;
}

@Component({
    components: {
        EstimatedCostsAndCredits,
        DepositAndBilling,
        PaymentMethods,
        VDatepicker,
        DatePickerIcon,
        LowBalanceIcon,
        NegativeBalanceIcon,
    },
})
export default class BillingArea extends Vue {
    /**
     * Mounted lifecycle hook before initial render.
     * Fetches billing history and project limits.
     */
    public async beforeMount(): Promise<void> {
        if (this.noProjectOrApiKeys) {
            await this.$router.push(RouteConfig.OnboardingTour.path);

            return;
        }

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BILLING_HISTORY);
            if (this.$store.getters.canUserCreateFirstProject && !this.userHasOwnProject) {
                await this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
                await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
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
     * Holds start and end dates.
     */
    private readonly dateRange: DateRange;

    /**
     * Holds empty start time for DatePicker.
     */
    public readonly startTime: StartTime = new StartTime();

    public constructor() {
        super();

        const currentDate = new Date();
        const previousDate = new Date();
        previousDate.setUTCMonth(currentDate.getUTCMonth() - 1);

        this.dateRange = {
            startDate: previousDate,
            endDate: currentDate,
        };
    }

    /**
     * Lifecycle hook before changing location.
     * Returns component state to default.
     * @param to
     * @param from
     * @param next
     */
    public async beforeRouteLeave(to, from, next): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        const buttons = [...(document as HTMLDocument).querySelectorAll('.account-billing-area__title-area__options-area__option')];
        buttons.forEach(option => {
            option.classList.remove('active');
        });

        buttons[0].classList.add('active');
        next();
    }

    public $refs!: {
        datePicker: VDatepicker & ShowCheck;
    };

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
        return this.balance.sum > 0 && this.balance.sum < this.CRITICAL_AMOUNT;
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
        return new ProjectOwning(this.$store).userHasOwnProject();
    }

    /**
     * Sets billing state to current billing period.
     * @param event holds click event.
     */
    public async onCurrentPeriodClick(event: any): Promise<void> {
        this.onButtonClickAction(event);

        try {
            this.$segment.track(SegmentEvent.REPORT_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
                start_date: this.dateRange.startDate,
                end_date: this.dateRange.endDate,
            });
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project charges. ${error.message}`);
        }
    }

    /**
     * Sets billing state to previous billing period.
     * @param event holds click event.
     */
    public async onPreviousPeriodClick(event: any): Promise<void> {
        this.onButtonClickAction(event);

        try {
            this.$segment.track(SegmentEvent.REPORT_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
                start_date: this.dateRange.startDate,
                end_date: this.dateRange.endDate,
            });
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project charges. ${error.message}`);
        }
    }

    /**
     * Sets billing state to custom billing period.
     * @param event holds click event.
     */
    public onCustomDateClick(event: any): void {
        this.$refs.datePicker.showCheck();
        this.onButtonClickAction(event);
        this.$segment.track(SegmentEvent.REPORT_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
            start_date: this.dateRange.startDate,
            end_date: this.dateRange.endDate,
        });
    }

    /**
     * Callback for VDatePicker.
     * @param datesArray selected date range.
     */
    public async getDates(datesArray: Date[]): Promise<void> {
        const firstDate = new Date(datesArray[0]);
        const secondDate = new Date(datesArray[1]);
        const isInverted = firstDate > secondDate;

        const startDate = isInverted ? secondDate : firstDate;
        const endDate = isInverted ? firstDate : secondDate;

        const dateRange: DateRange = new DateRange(startDate, endDate);

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES, dateRange);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project charges. ${error.message}`);
        }
    }

    /**
     * Indicates if user has no project nor api keys.
     */
    private get noProjectOrApiKeys(): boolean {
        return !this.$store.getters.selectedProject.id || this.$store.state.apiKeysModule.page.apiKeys.length === 0;
    }

    /**
     * Changes buttons styling depends on selected status.
     * @param event holds click event
     */
    private onButtonClickAction(event: any): void {
        let eventTarget = event.target;

        if (eventTarget.children.length === 0) {
            eventTarget = eventTarget.parentNode;
        }

        if (eventTarget.classList.contains('active')) {
            return;
        }

        this.changeActiveClass(eventTarget);
    }

    /**
     * Adds event target active class.
     * @param target holds event target
     */
    private changeActiveClass(target: any): void {
        this.removeActiveClass();
        target.classList.add('active');
    }

    /**
     * Removes active class from all the event targets.
     */
    private removeActiveClass(): void {
        const buttons = [...(document as any).querySelectorAll('.account-billing-area__title-area__options-area__option')];
        buttons.forEach(option => {
            option.classList.remove('active');
        });
    }
}
</script>

<style scoped lang="scss">
    .account-billing-area {
        padding-bottom: 55px;

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

                &__free-credits,
                &__tokens {
                    font-size: 16px;
                    line-height: 19px;
                }

                &__free-credits {
                    margin-right: 50px;
                    color: #768394;
                }
            }

            &__options-area {
                display: flex;
                align-items: center;

                &__option {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    padding: 15px 20px;
                    background-color: #fff;
                    border-radius: 6px;
                    margin-left: 16px;
                    cursor: pointer;

                    &__label {
                        font-family: 'font_medium', sans-serif;
                        font-size: 14px;
                        line-height: 14px;
                        color: #384b65;
                    }

                    &.active {
                        background-color: #2683ff;

                        .account-billing-area__title-area__options-area__option__label {
                            color: #fff;
                        }

                        .account-billing-area__title-area__options-area__option__image {

                            .date-picker-svg-path {
                                fill: #fff !important;
                            }
                        }
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

    .datepicker {
        padding: 12px;
    }

    /deep/ .datepickbox {
        max-height: 0;
        max-width: 0;
    }
</style>
