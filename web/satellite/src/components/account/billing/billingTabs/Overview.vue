// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="total-cost">
            <div class="total-cost__header-container">
                <h3 class="total-cost__header-container__title">Total Cost</h3>
                <div class="total-cost__header-container__date"><CalendarIcon />&nbsp;&nbsp;{{ currentDate }}</div>
            </div>
            <div class="total-cost__card-container">
                <div class="total-cost__card">
                    <EstimatedChargesIcon class="total-cost__card__main-icon" />
                    <p class="total-cost__card__money-text">{{ priceSummary | centsToDollars }}</p>
                    <p class="total-cost__card__label-text">
                        Total Estimated Charges
                        <img
                            src="@/../static/images/common/smallGreyWhiteInfo.png"
                            alt="info icon"
                            @mouseenter="showChargesTooltip = true"
                            @mouseleave="showChargesTooltip = false"
                        >
                    </p>
                    <div
                        v-if="showChargesTooltip"
                        class="total-cost__card__charges-tooltip"
                    >
                        <span class="total-cost__card__charges-tooltip__tooltip-text">If you still have Storage and Bandwidth remaining in your free tier, you won't be charged. This information is to help you estimate what the charges would have been had you graduated to the paid tier.</span>
                    </div>
                    <p
                        class="total-cost__card__link-text"
                        @click="routeToBillingHistory"
                    >
                        View Billing History →
                    </p>
                </div>
                <div class="total-cost__card">
                    <AvailableBalanceIcon class="total-cost__card__main-icon" />
                    <p class="total-cost__card__money-text">${{ balance.coins }}</p>
                    <p class="total-cost__card__label-text">STORJ Token Balance</p>
                    <p
                        class="total-cost__card__link-text"
                        @click="balanceClicked"
                    >
                        {{ hasZeroCoins ? "Add Funds" : "See Balance" }} →
                    </p>
                </div>

                <div v-if="balance.hasCredits()" class="total-cost__card">
                    <AvailableBalanceIcon class="total-cost__card__main-icon" />
                    <p class="total-cost__card__money-text">{{ balance.formattedCredits }}</p>
                    <p class="total-cost__card__label-text">Legacy STORJ Payments and Bonuses</p>
                </div>
            </div>
        </div>
        <div class="cost-by-project">
            <h3 class="cost-by-project__title">Cost by Project</h3>
            <div class="cost-by-project__buttons">
                <v-button
                    label="Edit Payment Method"
                    font-size="13px"
                    width="auto"
                    height="30px"
                    icon="lock"
                    :is-transparent="true"
                    class="cost-by-project__buttons__none-assigned"
                    :on-press="routeToPaymentMethods"
                />
                <v-button
                    label="See Payments"
                    font-size="13px"
                    width="auto"
                    height="30px"
                    icon="document"
                    :is-transparent="true"
                    class="cost-by-project__buttons__none-assigned"
                    :on-press="routeToBillingHistory"
                />
            </div>
            <div class="usage-charges-item-container__detailed-info-container__footer__buttons">
                <UsageAndChargesItem
                    v-for="usageAndCharges in projectUsageAndCharges"
                    :key="usageAndCharges.projectId"
                    :item="usageAndCharges"
                    class="cost-by-project__item"
                />
            </div>
            <router-view />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { RouteConfig } from '@/router';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { AccountBalance, ProjectUsageAndCharges } from '@/types/payments';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify, useRouter, useStore } from '@/utils/hooks';

import UsageAndChargesItem from '@/components/account/billing/billingTabs/UsageAndChargesItem.vue';
import VButton from '@/components/common/VButton.vue';

import EstimatedChargesIcon from '@/../static/images/account/billing/totalEstimatedChargesIcon.svg';
import AvailableBalanceIcon from '@/../static/images/account/billing/availableBalanceIcon.svg';
import CalendarIcon from '@/../static/images/account/billing/calendar-icon.svg';

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const store = useStore();
const notify = useNotify();
const router = useRouter();

const showChargesTooltip = ref<boolean>(false);
const isDataFetching = ref<boolean>(true);
const currentDate = ref<string>('');

/**
 * Returns account balance from store.
 */
const balance = computed((): AccountBalance => {
    return store.state.paymentsModule.balance;
});

/**
 * Returns whether the user's STORJ balance is empty.
 */
const hasZeroCoins = computed((): boolean => {
    return balance.value.coins === 0;
});

/**
 * projectUsageAndCharges is an array of all stored ProjectUsageAndCharges.
 */
const projectUsageAndCharges = computed((): ProjectUsageAndCharges[] => {
    return store.state.paymentsModule.usageAndCharges;
});

/**
 * priceSummary returns price summary of usages for all the projects.
 */
const priceSummary = computed((): number => {
    return store.state.paymentsModule.priceSummary;
});

function routeToBillingHistory(): void {
    analytics.eventTriggered(AnalyticsEvent.SEE_PAYMENTS_CLICKED);
    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingHistory).path);
}

function routeToPaymentMethods(): void {
    analytics.eventTriggered(AnalyticsEvent.EDIT_PAYMENT_METHOD_CLICKED);
    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).path);
}

function balanceClicked(): void {
    router.push({
        name: RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).name,
        params: { action: hasZeroCoins.value ? 'add tokens' : 'token history' },
    });
}

/**
 * Lifecycle hook after initial render.
 * Fetches projects and usage rollup.
 */
onMounted(async () => {
    try {
        await store.dispatch(PROJECTS_ACTIONS.FETCH);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_OVERVIEW_TAB);
        isDataFetching.value = false;
        return;
    }

    try {
        await store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        await store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_PRICE_MODEL);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_OVERVIEW_TAB);
    }

    isDataFetching.value = false;

    const rawDate = new Date();
    let currentYear = rawDate.getFullYear();
    currentDate.value = `${SHORT_MONTHS_NAMES[rawDate.getMonth()]} ${currentYear}`;
});
</script>

<style scoped lang="scss">
    .total-cost {
        font-family: 'font_regular', sans-serif;
        margin: 20px 0;

        &__header-container {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__date {
                display: flex;
                justify-content: space-between;
                align-items: flex-end;
                color: var(--c-grey-6);
                font-family: 'font_bold', sans-serif;
                border-radius: 5px;
                height: 15px;
                width: auto;
                padding: 10px;
            }
        }

        &__card-container {
            display: grid;
            grid-template-columns: 1fr 1fr 1fr;
            gap: 10px;
            margin-top: 20px;

            @media screen and (max-width: 786px) {
                grid-template-columns: 1fr 1fr;
            }

            @media screen and (max-width: 425px) {
                grid-template-columns: auto;
            }
        }

        &__card {
            overflow: hidden;
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            border-radius: 10px;
            background-color: #fff;
            padding: 20px;
            display: flex;
            flex-direction: column;
            justify-content: left;
            position: relative;

            &__money-text {
                font-weight: 800;
                font-size: 32px;
                margin-top: 10px;
            }

            &__label-text {
                font-weight: 400;
                margin-top: 10px;
                min-width: 200px;
            }

            &__link-text {
                font-weight: 500;
                text-decoration: underline;
                margin-top: 10px;
                cursor: pointer;
            }

            &__main-icon {

                :deep(g) {
                    filter: none;
                }
            }

            &__charges-tooltip {
                top: 5px;
                left: 86px;

                @media screen and (max-width: 635px) {
                    top: 5px;
                    left: -21px;
                }

                position: absolute;
                background: var(--c-grey-6);
                border-radius: 6px;
                width: 253px;
                color: #fff;
                display: flex;
                flex-direction: row;
                align-items: flex-start;
                padding: 8px;
                z-index: 1;
                transition: 250ms;

                &:after {
                    left: 50%;

                    @media screen and (max-width: 635px) {
                        left: 90%;
                    }

                    top: 100%;
                    content: '';
                    position: absolute;
                    bottom: 0;
                    width: 0;
                    height: 0;
                    border: 6px solid transparent;
                    border-top-color: var(--c-grey-6);
                    border-bottom: 0;
                    margin-left: -20px;
                    margin-bottom: -20px;
                }

                &__tooltip-text {
                    text-align: center;
                    font-weight: 500;
                }
            }
        }
    }

    .cost-by-project {
        font-family: 'font_regular', sans-serif;

        &__title {
            padding-bottom: 10px;
        }

        &__buttons {
            display: flex;
            align-self: center;
            flex-wrap: wrap;

            &__none-assigned {
                padding: 5px 10px;
                margin-right: 5px;
            }
        }
    }
</style>
