// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="period-selection" @click.stop="toggleDropdown">
        <div class="period-selection__current-choice">
            <div class="period-selection__current-choice__label-area">
                <DatePickerIcon />
                <span class="period-selection__current-choice__label-area__label">{{ currentOption }}</span>
            </div>
            <ExpandIcon v-if="!isDropdownShown" />
            <HideIcon v-else />
        </div>
        <div v-show="isDropdownShown" v-click-outside="closeDropdown" class="period-selection__dropdown">
            <div
                v-for="(option, index) in periodOptions"
                :key="index"
                class="period-selection__dropdown__item"
                @click.prevent.stop="select(option)"
            >
                <SelectedIcon v-if="isOptionSelected(option)" class="selected-image" />
                <span class="period-selection__dropdown__item__label">{{ option }}</span>
            </div>
            <div class="period-selection__dropdown__link-container" @click="redirect">
                <span class="period-selection__dropdown__link-container__link">Billing History</span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AnalyticsHttpApi } from '@/api/analytics';

import DatePickerIcon from '@/../static/images/account/billing/datePicker.svg';
import SelectedIcon from '@/../static/images/account/billing/selected.svg';
import ExpandIcon from '@/../static/images/common/BlueExpand.svg';
import HideIcon from '@/../static/images/common/BlueHide.svg';

// @vue/component
@Component({
    components: {
        DatePickerIcon,
        HideIcon,
        ExpandIcon,
        SelectedIcon,
    },
})

export default class PeriodSelection extends Vue {
    public readonly periodOptions: string[] = [
        'Current Billing Period',
        'Previous Billing Period',
    ];
    public currentOption: string = this.periodOptions[0];

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Indicates if periods dropdown is shown.
     */
    public get isDropdownShown(): Date {
        return this.$store.state.appStateModule.appState.isPeriodsDropdownShown;
    }

    /**
     * Returns start date of billing period from store.
     */
    public get startDate(): Date {
        return this.$store.state.paymentsModule.startDate;
    }

    /**
     * Returns end date of billing period from store.
     */
    public get endDate(): Date {
        return this.$store.state.paymentsModule.endDate;
    }

    /**
     * Indicates if option is selected.
     * @param option - option string.
     */
    public isOptionSelected(option: string): boolean {
        return option === this.currentOption;
    }

    /**
     * Holds logic for select option click.
     * @param option - option string.
     */
    public async select(option: string): Promise<void> {
        if (option === this.periodOptions[0]) {
            await this.onCurrentPeriodClick();
        }

        if (option === this.periodOptions[1]) {
            await this.onPreviousPeriodClick();
        }

        this.currentOption = option;
        this.closeDropdown();
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        if (!this.isDropdownShown) return;

        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Toggles dropdown visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PERIODS_DROPDOWN);
    }

    /**
     * Holds logic to redirect user to billing history page.
     */
    public redirect(): void {
        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.BillingHistory).path);
        this.$router.push(RouteConfig.Account.with(RouteConfig.BillingHistory).path);
    }

    /**
     * Sets billing state to previous billing period.
     */
    public async onPreviousPeriodClick(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project charges. ${error.message}`);
        }
    }

    /**
     * Sets billing state to current billing period.
     */
    public async onCurrentPeriodClick(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project charges. ${error.message}`);
        }
    }
}
</script>

<style scoped lang="scss">
    .period-selection {
        padding: 15px;
        width: 260px;
        background-color: #fff;
        position: relative;
        font-family: 'font_regular', sans-serif;
        border-radius: 6px;
        cursor: pointer;

        &__current-choice {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__label-area {
                display: flex;
                align-items: center;

                &__label {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 16px;
                    margin-left: 15px;
                }
            }
        }

        &__dropdown {
            z-index: 120;
            position: absolute;
            left: 0;
            top: 55px;
            background-color: #fff;
            border-radius: 6px;
            border: 1px solid #c5cbdb;
            box-shadow: 0 8px 34px rgb(161 173 185 / 41%);
            width: 290px;

            &__item {
                padding: 15px;

                &__label {
                    font-size: 14px;
                    line-height: 19px;
                    color: #494949;
                }

                &:hover {
                    background-color: #f5f5f7;
                }
            }

            &__link-container {
                width: calc(100% - 30px);
                height: 50px;
                padding: 0 15px;
                display: flex;
                align-items: center;

                &:hover {
                    background-color: #f5f5f7;
                }

                &__link {
                    font-size: 14px;
                    line-height: 19px;
                    color: #7e8b9c;
                }
            }
        }
    }

    .selected-image {
        margin-right: 10px;
    }
</style>
