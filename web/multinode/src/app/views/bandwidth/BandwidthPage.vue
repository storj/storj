// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bandwidth">
        <h1 class="bandwidth__title">Bandwidth & Disk</h1>
        <div class="bandwidth__dropdowns">
            <node-selection-dropdown />
            <satellite-selection-dropdown />
        </div>
        <div class="chart-container bandwidth-chart">
            <div class="chart-container__title-area">
                <p class="chart-container__title-area__title">Bandwidth Used This Month</p>
                <div class="chart-container__title-area__buttons-area">
                    <button
                        name="Show Bandwidth Chart"
                        class="chart-container__title-area__chart-choice-item"
                        type="button"
                        :class="{ 'active': (!isEgressChartShown && !isIngressChartShown) }"
                        @click.stop="openBandwidthChart"
                    >
                        Bandwidth
                    </button>
                    <button
                        name="Show Egress Chart"
                        class="chart-container__title-area__chart-choice-item"
                        type="button"
                        :class="{ 'active': isEgressChartShown }"
                        @click.stop="openEgressChart"
                    >
                        Egress
                    </button>
                    <button
                        name="Show Ingress Chart"
                        class="chart-container__title-area__chart-choice-item"
                        type="button"
                        :class="{ 'active': isIngressChartShown }"
                        @click.stop="openIngressChart"
                    >
                        Ingress
                    </button>
                </div>
            </div>
            <p v-if="isEgressChartShown" class="chart-container__amount"><b>{{ Size.toBase10String(bandwidth.egressSummary) }}</b></p>
            <p v-else-if="isIngressChartShown" class="chart-container__amount"><b>{{ Size.toBase10String(bandwidth.ingressSummary) }}</b></p>
            <p v-else class="chart-container__amount"><b>{{ Size.toBase10String(bandwidth.bandwidthSummary) }}</b></p>
            <div ref="chart" class="chart-container__chart">
                <egress-chart v-if="isEgressChartShown" :height="chartHeight" :width="chartWidth" :is-dark-mode="isDarkMode" />
                <ingress-chart v-else-if="isIngressChartShown" :height="chartHeight" :width="chartWidth" :is-dark-mode="isDarkMode" />
                <bandwidth-chart v-else :height="chartHeight" :width="chartWidth" :is-dark-mode="isDarkMode" />
            </div>
        </div>
        <section class="bandwidth__chart-area">
            <section class="chart-container">
                <div class="chart-container__title-area disk-space-title">
                    <p class="chart-container__title-area__title">Average Disk Space Used This Month</p>
                </div>
                <p class="chart-container__amount disk-space-amount"><b>{{ Size.toBase10String(diskSpaceUsageSummary) }}</b></p>
                <div ref="diskSpaceChart" class="chart-container__chart">
                    <DiskSpaceChart :height="diskSpaceChartHeight" :width="diskSpaceChartWidth" :is-dark-mode="isDarkMode" />
                </div>
            </section>
            <section class="disk-stat-chart">
                <disk-stat-chart />
            </section>
        </section>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useTheme } from 'vuetify';

import { UnauthorizedError } from '@/api';
import { BandwidthTraffic } from '@/bandwidth';
import { Size } from '@/private/memory/size';
import { useStorageStore } from '@/app/store/storageStore';
import { useBandwidthStore } from '@/app/store/bandwidthStore';
import { useNodesStore } from '@/app/store/nodesStore';

import BandwidthChart from '@/app/components/bandwidth/BandwidthChart.vue';
import EgressChart from '@/app/components/bandwidth/EgressChart.vue';
import IngressChart from '@/app/components/bandwidth/IngressChart.vue';
import NodeSelectionDropdown from '@/app/components/common/NodeSelectionDropdown.vue';
import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import DiskSpaceChart from '@/app/components/storage/DiskSpaceChart.vue';
import DiskStatChart from '@/app/components/storage/DiskStatChart.vue';

const theme = useTheme();

const storageStore = useStorageStore();
const bandwidthStore = useBandwidthStore();
const nodesStore = useNodesStore();

const chartWidth = ref(0);
const chartHeight = ref(0);
const diskSpaceChartWidth = ref(0);
const diskSpaceChartHeight = ref(0);
const isEgressChartShown = ref(false);
const isIngressChartShown = ref(false);

const chart = ref<HTMLElement>();
const diskSpaceChart = ref<HTMLElement>();

const bandwidth = computed<BandwidthTraffic>(() => bandwidthStore.state.traffic);
const diskSpaceUsageSummary = computed<number>(() => storageStore.state.usage.diskSpaceSummaryBytes);
const isDarkMode = computed<boolean>(() => theme.global.current.value.dark);

function recalculateChartDimensions(): void {
    if (chart.value) {
        chartWidth.value = chart.value.clientWidth;
        chartHeight.value = chart.value.clientHeight;
    }
    if (diskSpaceChart.value) {
        diskSpaceChartWidth.value = diskSpaceChart.value.clientWidth;
        diskSpaceChartHeight.value = diskSpaceChart.value.clientHeight;
    }
}

function openBandwidthChart(): void {
    isEgressChartShown.value = false;
    isIngressChartShown.value = false;
}

function openIngressChart(): void {
    isEgressChartShown.value = false;
    isIngressChartShown.value = true;
}

function openEgressChart(): void {
    isEgressChartShown.value = true;
    isIngressChartShown.value = false;
}

async function fetchTraffic(): Promise<void> {
    try {
        await Promise.all([
            bandwidthStore.fetch(),
            storageStore.usage(),
            storageStore.diskSpace(),
        ]);
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }
}

onMounted(async () => {
    window.addEventListener('resize', recalculateChartDimensions);

    try {
        await nodesStore.fetchOnline();
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }

    await fetchTraffic();

    nodesStore.$onAction(({ name, after }) => {
        if (name === 'selectSatellite' || name === 'selectNode') {
            after(async (_) => {
                await fetchTraffic();
            });
        }
    });

    recalculateChartDimensions();
});

onBeforeUnmount(() => {
    window.removeEventListener('resize', recalculateChartDimensions);
});
</script>

<style lang="scss" scoped>
    .bandwidth {
        box-sizing: border-box;
        padding: 60px;
        height: 100%;
        overflow-y: auto;
        background-color: var(--v-background-base);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--v-header-base);
            margin-bottom: 44px;
        }

        &__dropdowns {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 70%;

            & > *:first-of-type {
                margin-right: 20px;
            }

            .dropdown {
                max-width: unset;
            }
        }

        & .chart-container {
            box-sizing: border-box;
            width: 65%;
            height: 401px;
            background-color: var(--v-background-base);
            border: 1px solid var(--v-border-base);
            border-radius: 11px;
            padding: 32px 30px;
            margin: 20px 0 13px;
            position: relative;

            &__title-area {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__buttons-area {
                    display: flex;
                    flex-direction: row;
                    align-items: flex-end;
                }

                &__title {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    color: var(--v-header-base);
                    user-select: none;
                }

                &__chart-choice-item {
                    padding: 6px 8px;
                    background-color: var(--v-active2-base);
                    border-radius: 12px;
                    font-size: 12px;
                    color: var(--v-text-base);
                    max-height: 25px;
                    cursor: pointer;
                    user-select: none;
                    margin-left: 9px;
                    border: none;

                    &.active {
                        background-color: var(--v-active-base);
                        color: var(--v-header-base);
                    }
                }
            }

            &__amount {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 57px;
                color: var(--v-header-base);
            }

            &__chart {
                position: absolute;
                left: 0;
                width: calc(100% - 10px);
                height: 240px;
            }
        }

        &__chart-area {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            width: 100%;
        }
    }

    .disk-space-amount {
        margin-top: 5px;
    }

    .bandwidth-chart {
        width: 100% !important;
    }

    .disk-stat-chart {
        margin: 20px 0 13px;
        width: auto;
    }
</style>
