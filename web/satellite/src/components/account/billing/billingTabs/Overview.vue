// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="total-cost">
        <div class="total-cost__header-container">
            <h3 class="total-cost__header-container__title">Total Cost</h3>
            <div class="total-cost__header-container__date"><CalendarIcon />&nbsp;&nbsp;{{ currentDate }}</div>
        </div>
        <div class="total-cost__card-container">
            <div class="total-cost__card">
                <EstimatedChargesIcon class="total-cost__card__main-icon" />
                <p class="total-cost__card__money-text">{{ centsToDollars(priceSummary) }}</p>
                <p class="total-cost__card__label-text">
                    Total Estimated Usage
                    <VInfo class="total-cost__card__label-text__info">
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <span class="total-cost__card__label-text__info__inner">
                                This estimate includes all use before subtracting any discounts.
                                Pro accounts will only be charged for usage above the free tier limits,
                                and free accounts will not be charged.
                            </span>
                        </template>
                    </VInfo>
                </p>
                <p
                    class="total-cost__card__link-text"
                    @click="routeToBillingHistory"
                >
                    View Billing History →
                </p>
            </div>
            <div class="total-cost__card">
                <AvailableBalanceIcon class="total-cost__card__main-icon" />
                <p class="total-cost__card__money-text">{{ balance.formattedCoins }}</p>
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
        <div class="total-cost__report">
            <h3 class="total-cost__report__title">Detailed Usage Report</h3>
            <p class="total-cost__report__info">Get a complete usage report for all your projects.</p>
            <v-button
                class="total-cost__report__button"
                label="Download Report"
                icon="date"
                width="fit-content"
                height="30px"
                is-transparent
                :on-press="downloadUsageReport"
            />
        </div>
    </div>
    <div v-if="isDataFetching">
        <v-loader />
    </div>
    <div v-else class="cost-by-project">
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
        <UsageAndChargesItem
            v-for="id in projectIDs"
            :key="id"
            :project-id="id"
            class="cost-by-project__item"
        />
        <router-view />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { centsToDollars } from '@/utils/strings';
import { RouteConfig } from '@/types/router';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { AccountBalance } from '@/types/payments';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { MODALS } from '@/utils/constants/appStatePopUps';

import UsageAndChargesItem from '@/components/account/billing/billingTabs/UsageAndChargesItem.vue';
import VButton from '@/components/common/VButton.vue';
import VInfo from '@/components/common/VInfo.vue';
import VLoader from '@/components/common/VLoader.vue';

import EstimatedChargesIcon from '@/../static/images/account/billing/totalEstimatedChargesIcon.svg';
import AvailableBalanceIcon from '@/../static/images/account/billing/availableBalanceIcon.svg';
import CalendarIcon from '@/../static/images/account/billing/calendar-icon.svg';
import InfoIcon from '@/../static/images/billing/blueInfoIcon.svg';

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();

const notify = useNotify();
const router = useRouter();

const isDataFetching = ref<boolean>(true);
const currentDate = ref<string>('');

/**
 * Returns account balance from store.
 */
const balance = computed((): AccountBalance => {
    return billingStore.state.balance as AccountBalance;
});

/**
 * Returns whether the user's STORJ balance is empty.
 */
const hasZeroCoins = computed((): boolean => {
    return balance.value.coins === 0;
});

/**
 * projectIDs is an array of all of the project IDs for which there exist project usage charges.
 */
const projectIDs = computed((): string[] => {
    return projectsStore.state.projects
        .filter(proj => billingStore.state.projectCharges.hasProject(proj.id))
        .sort((proj1, proj2) => proj1.name.localeCompare(proj2.name))
        .map(proj => proj.id);
});

/**
 * priceSummary returns price summary of usages for all the projects.
 */
const priceSummary = computed((): number => {
    return billingStore.state.projectCharges.getPrice();
});

function routeToBillingHistory(): void {
    analyticsStore.eventTriggered(AnalyticsEvent.SEE_PAYMENTS_CLICKED);
    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingHistory).path);
}

function routeToPaymentMethods(): void {
    analyticsStore.eventTriggered(AnalyticsEvent.EDIT_PAYMENT_METHOD_CLICKED);
    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).path);
}

function balanceClicked(): void {
    router.push({
        name: RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).name,
        query: { action: hasZeroCoins.value ? 'add tokens' : 'token history' },
    });
}

/**
 * Handles download usage report click logic.
 */
function downloadUsageReport(): void {
    appStore.updateActiveModal(MODALS.detailedUsageReport);
}

/**
 * Lifecycle hook after initial render.
 * Fetches projects and usage rollup.
 */
onMounted(async () => {
    try {
        await Promise.all([
            projectsStore.getProjects(),
            billingStore.getBalance(),
        ]);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_OVERVIEW_TAB);
        isDataFetching.value = false;
        return;
    }

    try {
        await billingStore.getProjectUsagePriceModel();
        await billingStore.getProjectUsageAndChargesCurrentRollup();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_OVERVIEW_TAB);
    } finally {
        isDataFetching.value = false;
    }

    const rawDate = new Date();
    const currentYear = rawDate.getFullYear();
    currentDate.value = `${SHORT_MONTHS_NAMES[rawDate.getMonth()]} ${currentYear}`;
});
</script>

<style scoped lang="scss">
    .total-cost {
        font-family: 'font_regular', sans-serif;
        margin: 20px 0;

        &__report {
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            border-radius: 10px;
            background-color: #fff;
            padding: 20px;
            margin-top: 20px;

            &__title,
            &__info {
                margin-bottom: 10px;
            }

            &__button {
                padding: 0 16px;
            }
        }

        &__header-container {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__title {
                padding-bottom: 10px;
            }

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

            @media screen and (width <= 786px) {
                grid-template-columns: 1fr 1fr;
            }

            @media screen and (width <= 425px) {
                grid-template-columns: auto;
            }
        }

        &__card {
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
                display: flex;
                align-items: center;

                &__info {
                    margin-left: 8px;
                    max-height: 15px;

                    svg {
                        cursor: pointer;
                        width: 15px;
                        height: 15px;

                        :deep(path) {
                            fill: var(--c-black);
                        }
                    }

                    &__inner {
                        color: var(--c-white);
                    }
                }
            }

            &__link-text {
                width: fit-content;
                font-family: 'font-medium', sans-serif;
                text-decoration: underline;
                text-underline-position: under;
                margin-top: 10px;
                cursor: pointer;
            }

            &__main-icon {

                :deep(g) {
                    filter: none;
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

    :deep(.info__box) {
        width: 310px;
        left: calc(50% - 155px);
        top: unset;
        bottom: 15px;
        cursor: default;
        filter: none;
        transform: rotate(-180deg);

        @media screen and (width <= 385px) {
            left: calc(50% - 210px);
        }
    }

    :deep(.info__box__message) {
        background: var(--c-grey-6);
        border-radius: 6px;
        padding: 10px 8px;
        transform: rotate(-180deg);
    }

    :deep(.info__box__arrow) {
        background: var(--c-grey-6);
        width: 10px;
        height: 10px;
        margin-bottom: -3px;

        @media screen and (width <= 385px) {
            margin: 0 0 -3px -111px;
        }
    }
</style>
