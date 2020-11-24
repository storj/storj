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
            <p class="payout-area-container__section-title">Payout</p>
            <EstimationArea class="payout-area-container__estimation"/>
            <PayoutHistoryTable class="payout-area-container__payout-history-table" v-if="payoutPeriods.length > 0" />
            <p class="payout-area-container__section-title">Held Amount</p>
            <p class="additional-text">
                Learn more about held back
                <a
                    class="additional-text__link"
                    href="https://documentation.storj.io/resources/faq/held-back-amount"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    here
                </a>
            </p>
            <section class="payout-area-container__held-info-area">
                <TotalHeldArea v-if="isSatelliteSelected" />
                <div class="row" v-else >
                    <SingleInfo width="48%" label="Total Held Amount" :value="totalHeldAndPaid.held | centsToDollars" />
                    <SingleInfo width="48%" label="Total Held Returned" :value="totalHeldAndPaid.disposed | centsToDollars" />
                </div>
            </section>
            <HeldProgress v-if="isSatelliteSelected" class="payout-area-container__process-area" />
            <HeldHistoryArea />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EstimationArea from '@/app/components/payments/EstimationArea.vue';
import HeldHistoryArea from '@/app/components/payments/HeldHistoryArea.vue';
import HeldHistoryTable from '@/app/components/payments/HeldHistoryMonthlyBreakdownTable.vue';
import HeldProgress from '@/app/components/payments/HeldProgress.vue';
import PayoutHistoryTable from '@/app/components/payments/PayoutHistoryTable.vue';
import SingleInfo from '@/app/components/payments/SingleInfo.vue';
import TotalHeldArea from '@/app/components/payments/TotalHeldArea.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';

import BackArrowIcon from '@/../static/images/notifications/backArrow.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import { PayoutPeriod, TotalHeldAndPaid } from '@/storagenode/payouts/payouts';

@Component ({
    components: {
        TotalHeldArea,
        PayoutHistoryTable,
        HeldHistoryArea,
        HeldProgress,
        HeldHistoryTable,
        SingleInfo,
        SatelliteSelection,
        EstimationArea,
        BackArrowIcon,
    },
})
export default class PayoutArea extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Fetches payout information.
     */
    public async mounted(): Promise<any> {
        await this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, true);

        try {
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, 1);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_ESTIMATION, this.$store.state.node.selectedSatellite.id);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_PERIODS);
        } catch (error) {
            console.error(error);
        }

        await this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);
    }

    public get totalHeldAndPaid(): TotalHeldAndPaid {
        return this.$store.state.payoutModule.totalHeldAndPaid;
    }

    /**
     * Indicates if satellite is selected.
     */
    public get isSatelliteSelected(): boolean {
        return !!this.$store.state.node.selectedSatellite.id;
    }

    public get payoutPeriods(): PayoutPeriod[] {
        return this.$store.state.payoutModule.payoutPeriods;
    }
}
</script>

<style scoped lang="scss">
    .payout-area-container-overflow {
        position: relative;
        padding: 0 36px;
        width: calc(100% - 72px);
        overflow-y: scroll;
        overflow-x: hidden;
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

        &__held-info-area {
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

    @media screen and (max-width: 890px) {

        .payout-area-container {
            width: calc(100% - 36px - 36px);
            padding-left: 36px;
            padding-right: 36px;
        }
    }

    @media screen and (max-width: 640px) {

        .payout-area-container-overflow {
            padding: 0 15px 80px 15px;
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
