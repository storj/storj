// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <p class="ingress-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            :key="chartKey"
            chart-id="ingress-chart"
            :chart-data="chartData"
            :width="width"
            :height="height"
            :tooltip-constructor="ingressTooltip"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ChartData, ChartType, TooltipModel } from 'chart.js';

import { Tooltip, TooltipParams } from '@/app/types/chart';
import { Chart as ChartUtils } from '@/app/utils/chart';
import { BandwidthRollup } from '@/bandwidth';
import { Size } from '@/private/memory/size';
import { useBandwidthStore } from '@/app/store/bandwidthStore';

import VChart from '@/app/components/common/VChart.vue';

/**
 * stores ingress data for ingress bandwidth chart's tooltip
 */
class IngressTooltip {
    public normalIngress: string;
    public repairIngress: string;
    public date: string;

    public constructor(bandwidth: BandwidthRollup) {
        this.normalIngress = Size.toBase10String(bandwidth.ingress.usage);
        this.repairIngress = Size.toBase10String(bandwidth.ingress.repair);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

const bandwidthStore = useBandwidthStore();

const props = defineProps<{
    width: number;
    height: number;
    isDarkMode: boolean;
}>();

const chartKey = ref<number>(0);

const allBandwidth = computed<BandwidthRollup[]>(() => ChartUtils.populateEmptyBandwidth(bandwidthStore.state.traffic.bandwidthDaily));

const chartDataDimension = computed<string>(() => {
    if (!bandwidthStore.state.traffic.bandwidthDaily.length) {
        return 'Bytes';
    }

    return ChartUtils.getChartDataDimension(allBandwidth.value.map((elem) => elem.ingress.repair + elem.ingress.usage));
});

const chartData = computed<ChartData>(() => {
    let data: number[] = [0];

    if (allBandwidth.value.length) {
        data = ChartUtils.normalizeChartData(allBandwidth.value.map(elem => elem.ingress.repair + elem.ingress.usage));
    }

    return {
        labels: ChartUtils.daysDisplayedOnChart(),
        datasets: [
            {
                data,
                fill: true,
                backgroundColor: props.isDarkMode ? '#f7e8cb' : '#fff4df',
                borderColor: props.isDarkMode ? '#ffad12' : '#e1a128',
                borderWidth: 1,
                pointHoverBorderWidth: 3,
                hoverRadius: 8,
                hitRadius: 8,
                pointRadius: 4,
                pointBorderWidth: 1,
            },
        ],
    };
});

function rebuildChart(): void {
    chartKey.value += 1;
}

function ingressTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'ingress-chart', 'ingress-tooltip',
        tooltipMarkUp(tooltipModel), 200, 94);

    Tooltip.custom(tooltipParams);
}

function tooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new IngressTooltip(allBandwidth.value[dataIndex]);

    return `<div class='ingress-tooltip-body'>
                <div class='ingress-tooltip-body__info'>
                    <p>USAGE</p>
                    <b class="ingress-tooltip-bold-text">${dataPoint.normalIngress}</b>
                </div>
                <div class='ingress-tooltip-body__info'>
                    <p>REPAIR</p>
                    <b class="ingress-tooltip-bold-text">${dataPoint.repairIngress}</b>
                </div>
            </div>
            <div class='ingress-tooltip-footer'>
                <p>${dataPoint.date}</p>
            </div>`;
}

watch([() => props.isDarkMode, chartData, () => props.width], rebuildChart);
</script>

<style lang="scss">
    .ingress-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--v-header-base);
            margin: 0 0 5px 31px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #ingress-tooltip {
        background: var(--v-background2-base);
        border: 1px solid var(--v-border-base);
        min-width: 190px;
        min-height: 170px;
        font-size: 12px;
        border-radius: 14px;
        font-family: 'font_bold', sans-serif;
        color: var(--v-header-base);
        pointer-events: none;
        z-index: 9999;
    }

    #ingress-tooltip-point {
        z-index: 9999;
    }

    .ingress-tooltip-body {
        margin: 8px;

        &__info {
            display: flex;
            border-radius: 12px;
            padding: 14px;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 14px;
            position: relative;
            font-family: 'font_bold', sans-serif;
        }
    }

    .ingress-tooltip-bold-text {
        color: var(--v-warning-base);
        font-size: 14px;
    }

    .ingress-tooltip-footer {
        position: relative;
        font-size: 12px;
        width: auto;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 10px 0 16px;
        color: var(--v-header-base);
        font-family: 'font_bold', sans-serif;
    }
</style>
