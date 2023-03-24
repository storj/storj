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

<script setup lang="ts">
import { computed, ref } from 'vue';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';

import DatePickerIcon from '@/../static/images/account/billing/datePicker.svg';
import SelectedIcon from '@/../static/images/account/billing/selected.svg';
import ExpandIcon from '@/../static/images/common/BlueExpand.svg';
import HideIcon from '@/../static/images/common/BlueHide.svg';

const periodOptions: string[] = [
    'Current Billing Period',
    'Previous Billing Period',
];

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const store = useStore();
const router = useRouter();
const notify = useNotify();

const currentOption = ref<string>(periodOptions[0]);

/**
 * Indicates if periods dropdown is shown.
 */
const isDropdownShown = computed((): boolean => {
    return store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.PERIODS;
});

/**
 * Returns start date of billing period from store.
 */
const startDate = computed((): Date => {
    return store.state.paymentsModule.startDate;
});

/**
 * Returns end date of billing period from store.
 */
const endDate = computed((): Date => {
    return store.state.paymentsModule.endDate;
});

/**
 * Indicates if option is selected.
 * @param option - option string.
 */
function isOptionSelected(option: string): boolean {
    return option === currentOption.value;
}

/**
 * Closes dropdown.
 */
function closeDropdown(): void {
    if (!isDropdownShown.value) return;

    store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
}

/**
 * Toggles dropdown visibility.
 */
function toggleDropdown(): void {
    store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.PERIODS);
}

/**
 * Holds logic to redirect user to billing history page.
 */
function redirect(): void {
    analytics.pageVisit(RouteConfig.Account.with(RouteConfig.BillingHistory).path);
    router.push(RouteConfig.Account.with(RouteConfig.BillingHistory).path);
}

/**
 * Sets billing state to previous billing period.
 */
async function onPreviousPeriodClick(): Promise<void> {
    try {
        await store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP);
    } catch (error) {
        await notify.error(`Unable to fetch project charges. ${error.message}`, AnalyticsErrorEventSource.BILLING_PERIODS_SELECTION);
    }
}

/**
 * Sets billing state to current billing period.
 */
async function onCurrentPeriodClick(): Promise<void> {
    try {
        await store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
    } catch (error) {
        await notify.error(`Unable to fetch project charges. ${error.message}`, AnalyticsErrorEventSource.BILLING_PERIODS_SELECTION);
    }
}

/**
 * Holds logic for select option click.
 * @param option - option string.
 */
async function select(option: string): Promise<void> {
    if (option === periodOptions[0]) {
        await onCurrentPeriodClick();
    }

    if (option === periodOptions[1]) {
        await onPreviousPeriodClick();
    }

    currentOption.value = option;
    closeDropdown();
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
