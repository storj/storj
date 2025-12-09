// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="disk-stat-area">
        <p class="disk-stat-area__title">Allocated Disk Space</p>
        <p class="disk-stat-area__amount">{{ Size.toBase10String(diskSpace.allocated) }}</p>
        <doughnut-chart chart-id="disk-stat-chart" :chart-data="chartData" />
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
                    <p class="disk-stat-area__info-area__item__labels-area__label">Available</p>
                </div>
                <p class="disk-stat-area__info-area__item__labels-area__amount">{{ Size.toBase10String(diskSpace.available) }}</p>
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
import { useTheme } from 'vuetify';

import { DiskSpace } from '@/storage';
import { Size } from '@/private/memory/size';
import { useStorageStore } from '@/app/store/storageStore';

import DoughnutChart from '@/app/components/common/DoughnutChart.vue';

const theme = useTheme();

const storageStore = useStorageStore();

const chartData = computed<ChartData>(() => {
    return {
        datasets: [
            {
                data: [
                    diskSpace.value.available,
                    diskSpace.value.used,
                    diskSpace.value.overused,
                ],
                backgroundColor: theme.global.current.value.dark ? ['#d4effa', '#0052FF', '#ff4747'] : ['#D6D6D6', '#0059D0', '#EB5757'],
            },
        ],
    };
});

const diskSpace = computed<DiskSpace>(() => storageStore.state.diskSpace);
</script>

<style lang="scss">
    .disk-stat-area {
        width: 400px;
        height: 401px;
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

    @media screen and (width <= 1000px) {

        .disk-stat-area {
            width: 230px;

            &__info-area {
                right: unset;
                top: 330px;
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
</style>
