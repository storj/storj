// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-area-container">
        <header class="payout-area-container__header">
            <router-link to="/" class="payout-area-container__header__back-link">
                <BackArrowIcon />
            </router-link>
            <p class="payout-area-container__header__text">Payout information</p>
        </header>
        <SatelliteSelection />
        <p class="payout-area-container__section-title">Payout</p>
        <EstimationArea class="payout-area-container__estimation" />
        <p class="payout-area-container__section-title">Held Amount</p>
        <p class="additional-text">Learn more about held back <a class="additional-text__link">here</a></p>
        <section class="payout-area-container__held-info-area">
            <SingleInfo width="48%" label="Held Amount Rate" value="25%" />
            <SingleInfo v-if="false" width="48%" label="Total Held Amount" value="$19.93" />
        </section>
        <HeldProgress class="payout-area-container__process-area" />
        <section class="payout-area-container__held-history-container">
            <div class="payout-area-container__held-history-container__header">
                <p class="payout-area-container__held-history-container__header__title">Held Amount history</p>
            </div>
            <div class="payout-area-container__held-history-container__divider"></div>
            <HeldHistoryTable />
        </section>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EstimationArea from '@/app/components/payments/EstimationArea.vue';
import HeldHistoryTable from '@/app/components/payments/HeldHistoryTable.vue';
import HeldProgress from '@/app/components/payments/HeldProgress.vue';
import SingleInfo from '@/app/components/payments/SingleInfo.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';

import BackArrowIcon from '@/../static/images/notifications/backArrow.svg';

@Component ({
    components: {
        HeldProgress,
        HeldHistoryTable,
        SingleInfo,
        SatelliteSelection,
        EstimationArea,
        BackArrowIcon,
    },
})
export default class PayoutArea extends Vue {}
</script>

<style scoped lang="scss">
    .payout-area-container {
        width: 822px;
        font-family: 'font_regular', sans-serif;

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
                color: #535f77;
                margin-left: 29px;
                text-align: center;
            }
        }

        &__section-title {
            margin-top: 40px;
            font-size: 18px;
            color: #535f77;
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

        &__held-info-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            margin-top: 20px;
        }

        &__process-area {
            margin-top: 12px;
        }

        &__held-history-container {
            display: flex;
            flex-direction: column;
            padding: 28px 40px 10px 40px;
            background: #fff;
            border: 1px solid #eaeaea;
            box-sizing: border-box;
            border-radius: 12px;
            margin: 12px 0 50px;

            &__header {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 18px;
                    color: #535f77;
                }
            }

            &__divider {
                width: 100%;
                height: 1px;
                margin-top: 18px;
                background-color: #eaeaea;
            }
        }
    }

    .additional-text {
        margin-top: 5px;
        font-size: 14px;
        line-height: 17px;
        color: #848fa2;

        &__link {
            color: #2683ff;
            cursor: pointer;
            text-decoration: underline;
        }
    }
</style>
