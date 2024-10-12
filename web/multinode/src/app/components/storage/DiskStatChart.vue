// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="disk-stat-area">
        <p class="disk-stat-area__title">Total Disk Space</p>
        <p class="disk-stat-area__amount">{{ diskSpace.allocated | bytesToBase10String }}</p>
        <doughnut-chart class="disk-stat-area__chart" :chart-data="chartData" />
        <div class="disk-stat-area__info-area">
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle used" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Used</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ diskSpace.usedPieces | bytesToBase10String }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle free" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Free</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ freeSpace | bytesToBase10String }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle trash" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Trash</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ diskSpace.usedTrash | bytesToBase10String }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle overused" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Overused</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ diskSpace.overused | bytesToBase10String }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { DiskStatChartData, DiskStatDataSet } from '@/app/types/chart';
import { DiskSpace } from '@/storage';

import DoughnutChart from '@/app/components/common/DoughnutChart.vue';

// @vue/component
@Component({
    components: { DoughnutChart },
})
export default class DiskStatChart extends Vue {
    /**
     * Holds datasets for chart.
     */
    public get chartData(): DiskStatChartData {
        return new DiskStatChartData([
            new DiskStatDataSet(
                '',
                this.$vuetify.theme.dark ? ['#d4effa', '#0052FF', '#9dc6fc', '#ff4747'] : ['#D6D6D6', '#0059D0', '#8FA7C6', '#EB5757'],
                [
                    this.freeSpace,
                    this.diskSpace.usedPieces,
                    this.diskSpace.usedTrash,
                    this.diskSpace.overused,
                ],
                this.$vuetify.theme.dark ? '#242d40' : '#ffffff',
                2
            ),
        ]);
    }

    /**
     * Returns disk space information from store.
     */
    public get diskSpace(): DiskSpace {
        return this.$store.state.storage.diskSpace;
    }

    /**
     * Returns free space in allocated disk space.
     */
    public get freeSpace(): number {
        return this.diskSpace.available;
    }
}
</script>

<style lang="scss">
    .disk-stat-area {
        width: 400px;
        height: 336px;
        background-color: var(--v-background-base);
        border: 1px solid var(--v-border-base);
        border-radius: 11px;
        padding: 32px 20px;
        position: relative;

        &__title {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            color: var(--v-text-base);
            user-select: none;
        }

        &__amount {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 57px;
            color: var(--v-header-base);
            margin-top: 5px;
        }

        &__chart {
            position: absolute;
            width: calc(58% - 25px);
            height: 220px;
            top: 135px;
        }

        &__info-area {
            position: absolute;
            right: 30px;
            top: 60%;
            transform: translateY(-50%);
            width: calc(40% - 35px);
            display: flex;
            flex-direction: column;
            box-sizing: border-box;

            &__item {
                display: flex;
                justify-content: space-between;
                flex-direction: column;
                margin-top: 19px;

                &:first-of-type {
                    margin-top: 0;
                }

                &__labels-area {
                    display: flex;
                    align-items: center;

                    &__circle {
                        width: 14px;
                        height: 14px;
                        border-radius: 50%;
                        margin-right: 8px;
                    }

                    &__label {
                        font-family: 'font_regular', sans-serif;
                        font-size: 14px;
                        color: var(--v-text-base);
                    }

                    &__amount {
                        font-family: 'font_bold', sans-serif;
                        font-weight: bold;
                        font-size: 14px;
                        color: var(--v-header-base);
                        margin-left: 22px;
                        margin-top: 6px;
                    }
                }
            }
        }
    }

    .used {
        background: var(--v-primary-base);
    }

    .free {
        background: var(--v-free-base);
    }

    .trash {
        background: var(--v-trash-base);
    }

    .overused {
        background: var(--v-overused-base);
    }

    @media screen and (max-width: 1000px) {

        .disk-stat-area {
            width: calc(100% - 60px);

            &__chart {
                width: 250px;
                height: 250px;
                margin-left: 100px;
                top: 100px;
            }

            &__info-area {
                right: 120px;
                width: 185px;

                &__item {
                    flex-direction: row;

                    &__labels-area__amount {
                        margin: 0;
                    }
                }
            }
        }
    }

    @media screen and (max-width: 780px) {

        .disk-stat-area {

            &__chart {
                margin-left: 50px;
            }

            &__info-area {
                right: 90px;
                width: 140px;

                &__item {
                    flex-direction: row;

                    &__labels-area__amount {
                        margin: 0;
                    }
                }
            }
        }
    }

    @media screen and (max-width: 640px) {

        .disk-stat-area {

            &__chart {
                top: 125px;
                width: 200px;
                height: 200px;
                margin-left: 50px;
            }

            &__info-area {
                top: 55%;
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
                top: 100px;
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
                width: 200px;
            }
        }
    }
</style>
