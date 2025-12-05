// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-area-container-overflow">
        <div class="payout-area-container">
            <header class="payout-area-container__header">
                <router-link to="/" class="payout-area-container__header__back-link">
                    <BackArrowIcon />
                </router-link>
                <p class="payout-area-container__header__text">Payout Information</p>
            </header>
            <SatelliteSelection />
            <p class="payout-area-container__section-title">Balance</p>
            <section class="payout-area-container__balance-area">
                <div class="row">
                    <SingleInfo width="48%" label="Undistributed payout" :value="centsToDollars(balance)" info-text="You need to earn the minimum withdrawal amount so that we can transfer the entire amount to the wallet at the end of the month, otherwise it will remain on your balance for the next month or until you accumulate the minimum withdrawal amount" />
                    <SingleInfo width="48%" label="Estimated earning this month" :value="centsToDollars(currentMonthExpectations)" info-text="Estimated payout at the end of the month. This is only an estimate and may not reflect actual payout amount." />
                </div>
            </section>
            <p class="payout-area-container__section-title">Payout</p>
            <EstimationArea class="payout-area-container__estimation" />
            <PayoutHistoryTable v-if="payoutPeriods.length > 0" class="payout-area-container__payout-history-table" />
            <p class="payout-area-container__section-title">Held Amount</p>
            <p class="additional-text">
                Learn more about held back
                <a
                    class="additional-text__link"
                    href="https://docs.storj.io/node/resources/faq/held-back-amount"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    here
                </a>
            </p>
            <section class="payout-area-container__held-info-area">
                <TotalHeldArea v-if="isSatelliteSelected" />
                <div v-else class="row">
                    <SingleInfo width="48%" label="Total Held Amount" :value="centsToDollars(totalPayments.held)" />
                    <SingleInfo width="48%" label="Total Held Returned" :value="centsToDollars(totalPayments.disposed)" />
                </div>
            </section>
            <HeldProgress v-if="isSatelliteSelected" class="payout-area-container__process-area" />
            <HeldHistoryArea v-if="heldHistory.length" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { PayoutPeriod, SatelliteHeldHistory, TotalPayments } from '@/storagenode/payouts/payouts';
import { centsToDollars } from '@/app/utils/payout';
import { usePayoutStore } from '@/app/store/modules/payoutStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';
import { useAppStore } from '@/app/store/modules/appStore';
import { useNotificationsStore } from '@/app/store/modules/notificationsStore';

import EstimationArea from '@/app/components/payments/EstimationArea.vue';
import HeldHistoryArea from '@/app/components/payments/HeldHistoryArea.vue';
import HeldProgress from '@/app/components/payments/HeldProgress.vue';
import PayoutHistoryTable from '@/app/components/payments/PayoutHistoryTable.vue';
import SingleInfo from '@/app/components/payments/SingleInfo.vue';
import TotalHeldArea from '@/app/components/payments/TotalHeldArea.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';

import BackArrowIcon from '@/../static/images/notifications/backArrow.svg';

const payoutStore = usePayoutStore();
const nodeStore = useNodeStore();
const appStore = useAppStore();
const notificationsStore = useNotificationsStore();

const totalPayments = computed<TotalPayments>(() => {
    return payoutStore.state.totalPayments as TotalPayments;
});

const isSatelliteSelected = computed<boolean>(() => {
    return !!nodeStore.state.selectedSatellite.id;
});

const payoutPeriods = computed<PayoutPeriod[]>(() => {
    return payoutStore.state.payoutPeriods;
});

const currentMonthExpectations = computed<number>(() => {
    return payoutStore.state.estimation.currentMonthExpectations;
});

const balance = computed<number>(() => {
    return payoutStore.state.totalPayments.balance;
});

const heldHistory = computed<SatelliteHeldHistory[]>(() => {
    return payoutStore.state.heldHistory as SatelliteHeldHistory[];
});

onMounted(async () => {
    appStore.setLoading(true);

    try {
        await nodeStore.selectSatellite();
    } catch (error) {
        console.error(error);
    }

    try {
        await notificationsStore.fetchNotifications(1);
    } catch (error) {
        console.error(error);
    }

    const selectedSatelliteId = nodeStore.state.selectedSatellite.id;

    try {
        await payoutStore.fetchEstimation(selectedSatelliteId);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchPricingModel(selectedSatelliteId);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchTotalPayments();
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.getPeriods();
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchHeldHistory();
    } catch (error) {
        console.error(error);
    }

    appStore.setLoading(false);
});
</script>

<style scoped lang="scss">
    .payout-area-container-overflow {
        position: relative;
        padding: 0 36px;
        width: calc(100% - 72px);
        overflow: hidden scroll;
        display: flex;
        justify-content: center;
    }

    .payout-area-container {
        position: relative;
        width: 822px;
        font-family: 'font_regular', sans-serif;
        height: 100%;

        &__header {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: flex-start;
            margin: 17px 0;

            &__back-link {
                width: 25px;
                height: 25px;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 57px;
                color: var(--regular-text-color);
                margin-left: 29px;
                text-align: center;
            }
        }

        &__section-title {
            margin-top: 40px;
            font-size: 18px;
            color: var(--title-text-color);
        }

        &__estimation {
            margin-top: 20px;
        }

        &__content-area {
            width: 100%;
            height: auto;
            max-height: 62vh;
            background-color: #f3f4f9;
            border-radius: 12px;
        }

        &__payout-history-table {
            margin-top: 20px;
        }

        &__held-info-area,
        &__balance-area {
            display: flex;
            flex-direction: row;
            align-items: flex-start;
            justify-content: space-between;
            margin-top: 20px;
        }

        &__process-area {
            margin-top: 12px;
        }
    }

    .additional-text {
        margin-top: 5px;
        font-size: 14px;
        line-height: 17px;
        color: var(--regular-text-color);

        &__link {
            color: var(--navigation-link-color);
            cursor: pointer;
            text-decoration: underline;
        }
    }

    .row {
        display: flex;
        justify-content: space-between;
        width: 100%;
    }

    @media screen and (width <= 890px) {

        .payout-area-container {
            width: calc(100% - 36px - 36px);
            padding-left: 36px;
            padding-right: 36px;
        }
    }

    @media screen and (width <= 640px) {

        .payout-area-container-overflow {
            padding: 0 15px 80px;
            width: calc(100% - 30px);
        }

        .payout-area-container {
            width: calc(100% - 20px - 20px);
            padding-left: 20px;
            padding-right: 20px;

            &__header {
                margin-top: 50px;
            }

            &__held-info-area {
                flex-direction: column;

                .info-container {
                    width: 100% !important;

                    &:first-of-type {
                        margin-bottom: 20px;
                    }
                }
            }
        }

        .row {
            flex-direction: column;
        }
    }
</style>
