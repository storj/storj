// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="current-month-area">
        <VLoader v-if="isDataFetching" class="consts-loader" />
        <template v-else>
            <h1 class="current-month-area__costs">{{ priceSummary | centsToDollars }}</h1>
            <span class="current-month-area__title">Estimated Charges for {{ chosenPeriod }}</span>
            <p class="current-month-area__info">
                If you still have Storage and Bandwidth remaining in your free tier, you won’t be charged. This information
                is to help you estimate what charges would have been had you graduated to the paid tier.
            </p>
            <div class="current-month-area__content">
                <p class="current-month-area__content__title">DETAILS</p>
                <UsageAndChargesItem
                    v-for="usageAndCharges in projectUsageAndCharges"
                    :key="usageAndCharges.projectId"
                    :item="usageAndCharges"
                    class="item"
                />
            </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectUsageAndCharges } from '@/types/payments';
import { MONTHS_NAMES } from '@/utils/constants/date';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify, useStore } from '@/utils/hooks';

import VLoader from '@/components/common/VLoader.vue';
import UsageAndChargesItem from '@/components/account/billing/estimatedCostsAndCredits/UsageAndChargesItem.vue';

const store = useStore();
const notify = useNotify();

const isDataFetching = ref<boolean>(true);

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

/**
 * chosenPeriod returns billing period chosen by user.
 */
const chosenPeriod = computed((): string => {
    const dateFromStore = store.state.paymentsModule.startDate;

    return `${MONTHS_NAMES[dateFromStore.getUTCMonth()]} ${dateFromStore.getUTCFullYear()}`;
});

/**
 * Lifecycle hook after initial render.
 * Fetches projects and usage rollup.
 */
onMounted(async () => {
    try {
        await store.dispatch(PROJECTS_ACTIONS.FETCH);
    } catch (error) {
        isDataFetching.value = false;
        notify.error(error.message, AnalyticsErrorEventSource.BILLING_ESTIMATED_COSTS_AND_CREDITS);
        return;
    }

    try {
        await store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        await store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_PRICE_MODEL);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_ESTIMATED_COSTS_AND_CREDITS);
    }

    isDataFetching.value = false;
});
</script>

<style scoped lang="scss">
    h1,
    h2,
    p,
    span {
        margin: 0;
        color: #354049;
    }

    .current-month-area {
        margin-bottom: 32px;
        padding: 40px 40px 0;
        background-color: #fff;
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__costs {
            font-size: 36px;
            line-height: 53px;
            color: #384b65;
            font-family: 'font_medium', sans-serif;
        }

        &__title {
            font-size: 16px;
            line-height: 24px;
            color: #909090;
        }

        &__info {
            font-size: 14px;
            line-height: 20px;
            color: #909090;
            margin: 15px 0 0;
        }

        &__content {
            margin-top: 35px;

            &__title {
                font-size: 16px;
                line-height: 23px;
                letter-spacing: 0.04em;
                text-transform: uppercase;
                color: #919191;
                margin-bottom: 25px;
            }

            &__usage-charges {
                margin: 18px 0 0;
                background-color: #f5f6fa;
                border-radius: 12px;
                cursor: pointer;
            }
        }
    }

    .item {
        border-top: 1px solid #c7cdd2;
    }

    .consts-loader {
        padding-bottom: 40px;
    }
</style>
