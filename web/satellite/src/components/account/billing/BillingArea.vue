// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-billing-area">
        <div class="account-billing-area__title-area">
            <h1 class="account-billing-area__title-area__title">Billing</h1>
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
        <div class="account-billing-area__notification-container" v-if="hasNoCreditCard">
            <div class="account-billing-area__notification-container__negative-balance" v-if="isBalanceNegative">
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="40" height="40" rx="10" fill="#EB5757"/>
                    <path d="M20.5 22.75C21.7676 22.75 22.8047 21.645 22.8047 20.2944V10.4556C22.8047 10.3328 22.797 10.2019 22.7816 10.0791C22.6126 8.90857 21.6523 8 20.5 8C19.2324 8 18.1953 9.10502 18.1953 10.4556V20.2862C18.1953 21.645 19.2324 22.75 20.5 22.75Z" fill="#F5F5F9"/>
                    <path d="M20.5 25.1465C18.7146 25.1465 17.2734 26.5877 17.2734 28.373C17.2734 30.1584 18.7146 31.5996 20.5 31.5996C22.2853 31.5996 23.7265 30.1584 23.7265 28.373C23.7337 26.5877 22.2925 25.1465 20.5 25.1465Z" fill="#F5F5F9"/>
                </svg>
                <p class="account-billing-area__notification-container__negative-balance__text">Your usage charges exceed your account balance. Please add STORJ Tokens or a debit/credit card to prevent data loss.</p>
            </div>
            <div class="account-billing-area__notification-container__low-balance" v-if="isBalanceLow">
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M37.0275 30.9607C36.5353 30.0514 36.04 29.1404 35.5463 28.2264C34.6275 26.531 33.7103 24.8357 32.7931 23.1404C31.6916 21.1091 30.5931 19.0748 29.4931 17.0436C28.4307 15.0826 27.3713 13.1248 26.3088 11.1656C25.5275 9.72492 24.7494 8.28276 23.9681 6.84076C23.7572 6.45014 23.5463 6.05952 23.3353 5.672C23.1166 5.26576 22.8853 4.87512 22.5541 4.54388C21.3979 3.3798 19.4791 3.15636 18.0885 4.03608C17.4916 4.41421 17.0604 4.95016 16.7291 5.5642C16.2213 6.50172 15.7135 7.4392 15.2057 8.37984C14.2807 10.0908 13.3541 11.8017 12.4291 13.5126C11.3151 15.5548 10.2104 17.6018 9.10102 19.6486C8.05102 21.5893 6.99946 23.5268 5.9479 25.469C5.17914 26.8909 4.40882 28.3097 3.63854 29.7314C3.43541 30.1064 3.2323 30.4814 3.02918 30.8564C2.7448 31.3846 2.52138 31.9189 2.45106 32.5283C2.25262 34.2502 3.45886 35.9471 5.12606 36.369C5.5667 36.4815 6.00106 36.4861 6.44638 36.4861H33.9464H33.9901C34.8964 36.4674 35.7558 36.1268 36.4198 35.5096C37.0604 34.9158 37.4354 34.1033 37.5417 33.2439C37.6432 32.4346 37.4089 31.6689 37.0276 30.9611L37.0275 30.9607ZM18.4367 13.9527C18.4367 13.0777 19.1523 12.4293 19.9992 12.3902C20.8429 12.3512 21.5617 13.1371 21.5617 13.9527V24.9559C21.5617 25.8309 20.846 26.4794 19.9992 26.5184C19.1554 26.5575 18.4367 25.7715 18.4367 24.9559V13.9527ZM19.9992 31.8403C19.1211 31.8403 18.4085 31.1294 18.4085 30.2497C18.4085 29.3716 19.1195 28.659 19.9992 28.659C20.8773 28.659 21.5898 29.37 21.5898 30.2497C21.5898 31.1278 20.8773 31.8403 19.9992 31.8403Z" fill="#F4D638"/>
                </svg>
                <p class="account-billing-area__notification-container__low-balance__text">Your account balance is running low. Please add STORJ Tokens or a debit/credit card to prevent data loss.</p>
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

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { DateRange } from '@/types/payments';
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
     * Indicates if isEstimatedCostsAndCredits component is visible.
     */
    public get isSummaryVisible(): boolean {
        const isBalancePositive: boolean = this.$store.state.paymentsModule.balance > 0;

        return isBalancePositive || this.userHasOwnProject;
    }

    /**
     * Indicates if no credit cards attached to account.
     */
    public get hasNoCreditCard(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
    }

    /**
     * Indicates balance is below zero.
     */
    public get isBalanceNegative(): boolean {
        return this.$store.state.paymentsModule.balance < 0;
    }

    /**
     * Indicates balance is not below zero but lower then CRITICAL_AMOUNT.
     */
    public get isBalanceLow(): boolean {
        return this.$store.state.paymentsModule.balance > 0 && this.$store.state.paymentsModule.balance < this.CRITICAL_AMOUNT;
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
            margin: 60px 0 35px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
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
            margin-top: 35px;

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

    .datepicker {
        padding: 12px;
    }

    /deep/ .datepickbox {
        max-height: 0;
        max-width: 0;
    }
</style>
