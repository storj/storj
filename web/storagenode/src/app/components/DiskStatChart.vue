// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="disk-stat-area">
        <p class="disk-stat-area__title">Total Disk Space</p>
        <p class="disk-stat-area__amount">{{ total }}</p>
        <DoughnutChart class="disk-stat-area__chart" :chart-data="chartData" />
        <div class="disk-stat-area__info-area">
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle used"></div>
                    <p class="disk-stat-area__info-area__item__labels-area__label">Used</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ used }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle free"></div>
                    <p class="disk-stat-area__info-area__item__labels-area__label">Free</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ free }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle trash"></div>
                    <p class="disk-stat-area__info-area__item__labels-area__label">Trash</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ trash }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DoughnutChart from '@/app/components/DoughnutChart.vue';

import {
    DiskStatChartData,
    DiskStatDataSet,
} from '@/app/types/chartData';
import { formatBytes } from '@/app/utils/converter';

@Component({
    components: {
        DoughnutChart,
    },
})
export default class DiskStatChart extends Vue {
    /**
     * Holds datasets for chart.
     */
    public get chartData(): DiskStatChartData {
        const diskSpace = this.$store.state.node.utilization.diskSpace;
        const free = diskSpace.available - diskSpace.used - diskSpace.trash;

        return new DiskStatChartData([
            new DiskStatDataSet(
            '',
                ['#D6D6D6', '#0059D0', '#8FA7C6'],
                [
                    free,
                    diskSpace.used,
                    diskSpace.trash,
                ],
            ),
        ]);
    }

    /**
     * Returns formatted used disk space amount.
     */
    public get total(): string {
        return formatBytes(this.$store.state.node.utilization.diskSpace.available);
    }

    /**
     * Returns formatted used disk space amount.
     */
    public get used(): string {
        return formatBytes(this.$store.state.node.utilization.diskSpace.used);
    }

    /**
     * Returns formatted available disk space amount.
     */
    public get free(): string {
        return formatBytes(
            this.$store.state.node.utilization.diskSpace.available -
            this.$store.state.node.utilization.diskSpace.used -
            this.$store.state.node.utilization.diskSpace.trash,
        );
    }

    /**
     * Returns formatted trash disk space amount.
     */
    public get trash(): string {
        return formatBytes(this.$store.state.node.utilization.diskSpace.trash);
    }
}
</script>

<style lang="scss">

    .disk-stat-area {
        width: 339px;
        height: 336px;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 11px;
        padding: 32px 30px;
        position: relative;

        &__title {
            font-size: 14px;
            color: var(--regular-text-color);
            user-select: none;
        }

        &__amount {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 57px;
            color: var(--regular-text-color);
            margin-top: 5px;
        }

        &__chart {
            position: absolute;
            left: 30px;
            width: calc(50% - 30px);
            height: 220px;
            margin-top: 35px;
        }

        &__info-area {
            position: absolute;
            right: 30px;
            top: 57%;
            transform: translateY(-50%);
            width: calc(50% - 50px);
            display: flex;
            flex-direction: column;

            &__item {
                display: flex;
                justify-content: space-between;
                margin-top: 19px;

                &:first-of-type {
                    margin-top: 0;
                }

                &__labels-area {
                    display: flex;

                    &__circle {
                        width: 14px;
                        height: 14px;
                        border-radius: 50%;
                        margin-right: 8px;
                    }

                    &__label {
                        font-family: 'font_regular', sans-serif;
                        font-size: 14px;
                        color: var(--disk-stat-chart-text-color);
                    }

                    &__amount {
                        font-family: 'font_bold', sans-serif;
                        font-weight: bold;
                        font-size: 14px;
                        color: var(--disk-stat-chart-text-color);
                    }
                }
            }
        }
    }

    .used {
        background: #0059d0;
    }

    .free {
        background: #d6d6d6;
    }

    .trash {
        background: #8fa7c6;
    }

    @media screen and (max-width: 1000px) {

        .disk-stat-area {
            width: calc(100% - 60px);

            &__chart {
                width: 250px;
                height: 250px;
                margin-left: 100px;
                margin-top: 0;
            }

            &__info-area {
                top: 60%;
                right: 120px;
                width: 185px;
            }
        }
    }

    @media screen and (max-width: 780px) {

        .disk-stat-area {

            &__chart {
                margin-left: 50px;
            }

            &__info-area {
                top: 60%;
                right: 90px;
                width: 140px;
            }
        }
    }

    @media screen and (max-width: 640px) {

        .disk-stat-area {

            &__chart {
                width: 200px;
                height: 200px;
                margin-left: 50px;
            }

            &__info-area {
                top: 50%;
                right: 90px;
                width: 140px;
            }
        }
    }

    @media screen and (max-width: 550px) {

        .disk-stat-area {
            height: 414px;
            width: calc(100% - 36px);
            padding: 24px 18px;

            &__chart {
                width: 200px;
                height: 200px;
                left: 50%;
                transform: translateX(-50%);
                margin: 0;
            }

            &__info-area {
                top: 70%;
                right: 50%;
                transform: translateX(50%);
                bottom: 10px;
                height: 100px;
            }
        }
    }
</style>
