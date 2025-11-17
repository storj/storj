// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="disk-stat-area">
        <p class="disk-stat-area__title">Allocated Disk Space</p>
        <p class="disk-stat-area__amount">{{ Size.toBase10String(diskSpace.allocated) }}</p>
        <DoughnutChart chart-id="disk-stat-chart" :chart-data="chartData" />
        <div class="disk-stat-area__info-area">
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle used" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Used</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ Size.toBase10String(diskSpace.used) }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle free" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Free</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ Size.toBase10String(free) }}</p>
            </div>
            <div class="disk-stat-area__info-area__item">
                <div class="disk-stat-area__info-area__item__labels-area">
                    <div class="disk-stat-area__info-area__item__labels-area__circle overused" />
                    <p class="disk-stat-area__info-area__item__labels-area__label">Overused</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ Size.toBase10String(diskSpace.overused) }}</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { ChartData } from 'chart.js';

import { Traffic } from '@/storagenode/sno/sno';
import { Size } from '@/private/memory/size';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import DoughnutChart from '@/app/components/DoughnutChart.vue';

const nodeStore = useNodeStore();

const chartData = computed<ChartData>(() => {
    return {
        labels: ['Available', 'Used other', 'Used trash', 'Used reclaimable', 'Reserved', 'Overused'],
        datasets: [
            {
                data: [
                    free.value,
                    diskSpace.value.used - diskSpace.value.trash - diskSpace.value.reclaimable,
                    diskSpace.value.trash,
                    diskSpace.value.reclaimable,
                    diskSpace.value.reserved,
                    diskSpace.value.overused,
                ],
                backgroundColor: ['#D6D6D6', '#0059D0', '#0059D0', '#0059D0', '#8FA7C6', '#2582FF'],
            },
        ],
    };
});

const diskSpace = computed<Traffic>(() => {
    return nodeStore.state.utilization.diskSpace;
});

const free = computed<number>(() => {
    let free = diskSpace.value.allocated - diskSpace.value.used - diskSpace.value.reserved;

    if (free < 0) free = 0;

    return free;
});
</script>

<style lang="scss">
    .disk-stat-area {
        width: 339px;
        height: 336px;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 11px;
        padding: 32px 20px;
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

        &__info-area {
            position: absolute;
            right: 10px;
            top: 55%;
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
                        color: var(--disk-stat-chart-text-color);
                    }

                    &__amount {
                        font-family: 'font_bold', sans-serif;
                        font-weight: bold;
                        font-size: 14px;
                        color: var(--disk-stat-chart-text-color);
                        margin-left: 22px;
                        margin-top: 6px;
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

    .overused {
        background: #2582ff;
    }

    @media screen and (width <= 1000px) {

        .disk-stat-area {
            width: calc(100% - 40px);

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

    @media screen and (width <= 780px) {

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

    @media screen and (width <= 640px) {

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

    @media screen and (width <= 550px) {

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
