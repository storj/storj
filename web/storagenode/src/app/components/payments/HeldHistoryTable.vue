// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="held-history-table-container__labels-area">
            <div class="column justify-start column-1">
                <p class="held-history-table-container__labels-area__text">Satellite</p>
            </div>
            <div class="column justify-start column-2">
                <p class="held-history-table-container__labels-area__text">Month 1-3</p>
            </div>
            <div class="column justify-start column-3">
                <p class="held-history-table-container__labels-area__text">Month 4-6</p>
            </div>
            <div class="column justify-end column-4">
                <p class="held-history-table-container__labels-area__text">Month 7-9</p>
            </div>
        </div>
        <div v-for="item in data" class="held-history-table-container__info-area">
            <div class="column justify-start column-1">
                <p class="held-history-table-container__info-area__text">{{ item.satelliteName }}</p>
                <p class="held-history-table-container__info-area__text">{{ item.satelliteAge }}</p>
            </div>
            <div class="column justify-start column-2">
                <p class="held-history-table-container__info-area__text">{{ item.firstPeriodAmount }}</p>
            </div>
            <div class="column justify-start column-3">
                <p class="held-history-table-container__info-area__text">{{ item.secondPeriodAmount }}</p>
            </div>
            <div class="column justify-end column-4">
                <p class="held-history-table-container__info-area__text">{{ item.thirdPeriodAmount }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

class HeldHistoryTableRow {
    public constructor(
        public satelliteName: string = '',
        public satelliteAge: string = '',
        public firstPeriodAmount: string = '',
        public secondPeriodAmount: string = '',
        public thirdPeriodAmount: string = '',
    ) {}
}

@Component
export default class HeldHistoryTable extends Vue {
    public data: HeldHistoryTableRow[] = [
        new HeldHistoryTableRow('us-central-1', '17 month', '$0.0005', '$0.0005', '$0.0005'),
        new HeldHistoryTableRow('europe-west-1', '7 month', '$0.0005', '$0.0005', '$0.0005'),
        new HeldHistoryTableRow('europe-west-1', '3 month', '$0.0005', '$0.0005', '$0.0005'),
    ];
}
</script>

<style scoped lang="scss">
    .held-history-table-container {

        &__labels-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            margin-top: 17px;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;
            background: #f9fafc;

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                color: #909bad;
            }
        }

        &__info-area {
            padding: 0 16px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            height: 56px;
            border-bottom: 1px solid #a9b5c1;

            &:last-of-type {
                border-bottom: none;
            }

            &__text {
                font-family: 'font_regular', sans-serif;
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
        width: 40%;
    }

    .column-2,
    .column-3,
    .column-4 {
        width: 13%;
    }
</style>
