// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="estimation-table-container__labels-area">
            <div class="column justify-start column-1">
                <p class="estimation-table-container__labels-area__text">Name</p>
            </div>
            <div class="column justify-start column-2">
                <p class="estimation-table-container__labels-area__text">Type</p>
            </div>
            <div class="column justify-start column-3">
                <p class="estimation-table-container__labels-area__text">Disk</p>
            </div>
            <div class="column justify-start column-4">
                <p class="estimation-table-container__labels-area__text">Bandwidth</p>
            </div>
            <div class="column justify-end column-5">
                <p class="estimation-table-container__labels-area__text">Payout</p>
            </div>
        </div>
        <div v-for="item in data" class="estimation-table-container__info-area">
            <div class="column justify-start column-1">
                <p class="estimation-table-container__info-area__text">{{ item.name }}</p>
            </div>
            <div class="column justify-start column-2">
                <p class="estimation-table-container__info-area__text">{{ item.type }}</p>
            </div>
            <div class="column justify-start column-3">
                <p class="estimation-table-container__info-area__text">{{ item.disk }}</p>
            </div>
            <div class="column justify-start column-4">
                <p class="estimation-table-container__info-area__text">{{ item.bandwidth }}</p>
            </div>
            <div class="column justify-end column-5">
                <p class="estimation-table-container__info-area__text">{{ item.payout }}</p>
            </div>
        </div>
        <div class="estimation-table-container__held-area">
            <p class="estimation-table-container__held-area__text">25% Held back</p>
            <p class="estimation-table-container__held-area__text">-$0.67</p>
        </div>
        <div class="estimation-table-container__total-area">
            <div class="column justify-start column-1">
                <p class="estimation-table-container__total-area__text">TOTAL</p>
            </div>
            <div class="column justify-start column-2"></div>
            <div class="column justify-start column-3">
                <p class="estimation-table-container__total-area__text">25.45 GBm</p>
            </div>
            <div class="column justify-start column-4">
                <p class="estimation-table-container__total-area__text">98.24 GB</p>
            </div>
            <div class="column justify-end column-5">
                <p class="estimation-table-container__total-area__text">$1.99</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

class EstimationTableRow {
    public constructor(
        public name: string = '',
        public type: string = '',
        public disk: string = '',
        public bandwidth: string = '',
        public payout: string = '',
    ) {}
}

@Component
export default class EstimationTable extends Vue {
    public data: EstimationTableRow[] = [
        new EstimationTableRow('Download', 'Egress', '--', '98.24 GB', '$1.94'),
        new EstimationTableRow('Download Repair', 'Egress', '--', '0.00 B', '$0.24'),
        new EstimationTableRow('Download Audit', 'Egress', '--', '22.53 KB', '$0.00'),
        new EstimationTableRow('Disk Average Month', 'Storage', '25.45 GBm', '--', '$0.48'),
    ];
}
</script>

<style scoped lang="scss">
    .estimation-table-container {
        font-family: 'font_regular', sans-serif;

        &__labels-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: center;
            margin-top: 17px;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;
            background: #f9fafc;

            &__text {
                font-weight: 500;
                font-size: 14px;
                color: #909bad;
            }
        }

        &__info-area {
            padding: 0 16px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: center;
            height: 56px;
            border-bottom: 1px solid #a9b5c1;

            &__text {
                font-size: 14px;
                color: #535f77;
            }
        }

        &__held-area {
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                color: #535f77;
            }
        }

        &__total-area {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;
            background: rgba(0, 117, 255, 0.05);

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                color: #535f77;
            }
        }
    }

    .column {
        display: flex;
        flex-direction: row;
        align-items: center;
    }

    .justify-start {
        justify-content: flex-start;
    }

    .justify-end {
        justify-content: flex-end;
    }

    .column-1 {
        width: 26.9%;
    }

    .column-2 {
        width: 18.3%;
    }

    .column-3 {
        width: 22.2%;
    }

    .column-4 {
        width: 23.9%;
    }

    .column-5 {
        width: 8.7%;
    }
</style>
