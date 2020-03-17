// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="estimation-container">
        <div class="estimation-container__header">
            <p class="estimation-container__header__title">Info & Estimation</p>
            <EstimationPeriodDropdown />
        </div>
        <div class="estimation-container__divider"></div>
        <EstimationTable />
        <div class="estimation-container__payout-area">
            <div class="estimation-container__payout-area__left-area">
                <p class="title-text">Estimated Payout</p>
                <p class="additional-text">At the end of the month if the load keeps the same for the rest of the month.</p>
            </div>
            <div class="estimation-container__payout-area__right-area">
                <p class="title-text">$25.93</p>
                <p class="additional-text">4 Mar 2020</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';
import EstimationTable from '@/app/components/payments/EstimationTable.vue';

@Component ({
    components: {
        EstimationTable,
        EstimationPeriodDropdown,
    },
})
export default class EstimationArea extends Vue {}
</script>

<style scoped lang="scss">
    .estimation-container {
        display: flex;
        flex-direction: column;
        padding: 28px 40px 0 40px;
        background: #fff;
        border: 1px solid #eaeaea;
        box-sizing: border-box;
        border-radius: 12px;
        font-family: 'font_regular', sans-serif;

        &__header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-weight: 500;
                font-size: 18px;
                color: #535f77;
            }
        }

        &__total-held,
        &__payout-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 87px;

            &__left-area {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
            }

            &__right-area {
                display: flex;
                flex-direction: column;
                align-items: flex-end;
            }
        }

        &__total-held {
            border-bottom: 1px solid #eaeaea;
        }

        &__divider {
            width: 100%;
            height: 1px;
            margin-top: 18px;
            background-color: #eaeaea;
        }
    }

    .title-text {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 20px;
        color: #535f77;
    }

    .additional-text {
        font-size: 13px;
        line-height: 17px;
        color: #b5bdcb;
    }
</style>
