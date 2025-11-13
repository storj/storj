// Copyright (C) 2019 Storj Labs, Inc.
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
import { ChartUtils } from '@/app/utils/chart';
import { Size } from '@/private/memory/size';
import { IngressUsed } from '@/storagenode/sno/sno';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import VChart from '@/app/components/VChart.vue';

/**
 * stores ingress data for ingress bandwidth chart's tooltip
 */
class IngressTooltip {
    public normalIngress: string;
    public repairIngress: string;
    public date: string;

    public constructor(bandwidth: IngressUsed) {
        this.normalIngress = Size.toBase10String(bandwidth.ingress.usage);
        this.repairIngress = Size.toBase10String(bandwidth.ingress.repair);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

const nodeStore = useNodeStore();

const props = defineProps<{
    width: number;
    height: number;
    isDarkMode: boolean;
}>();

const chartKey = ref<number>(0);

const chartBackgroundColor = computed<string>(() => {
    return props.isDarkMode ? '#E1A128' : '#fff4df';
});

const allBandwidth = computed<IngressUsed[]>(() => {
    return ChartUtils.populateEmptyBandwidth(nodeStore.state.ingressChartData, IngressUsed);
});

const chartDataDimension = computed<string>(() => {
    if (!nodeStore.state.ingressChartData.length) {
        return 'Bytes';
    }

    return ChartUtils.getChartDataDimension(allBandwidth.value.map((elem) => {
        return elem.summary();
    }));
});

const chartData = computed<ChartData>(() => {
    let data: number[] = [0];
    if (allBandwidth.value.length) {
        data = ChartUtils.normalizeChartData(allBandwidth.value.map(elem => elem.summary()));
    }

    return {
        labels: ChartUtils.daysDisplayedOnChart(),
        datasets: [
            {
                fill: true,
                backgroundColor: chartBackgroundColor.value,
                borderColor: '#e1a128',
                borderWidth: 1,
                pointHoverBorderWidth: 3,
                hoverRadius: 8,
                hitRadius: 8,
                pointRadius: 4,
                pointBorderWidth: 1,
                data,
            },
        ],
    };
});

function rebuildChart(): void {
    chartKey.value += 1;
}

function ingressTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'ingress-chart', 'ingress-tooltip',
        tooltipMarkUp(tooltipModel), 205, 94);

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
            color: var(--regular-text-color);
            margin: 0 0 5px 3px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #ingress-tooltip {
        background-image: var(--tooltip-background-path);
        background-repeat: no-repeat;
        background-size: cover;
        min-width: 190px;
        min-height: 170px;
        font-size: 12px;
        border-radius: 14px;
        color: #535f77;
        pointer-events: none;
        z-index: 9999;
    }

    #ingress-tooltip-arrow {
        background-image: var(--tooltip-arrow-path);
        background-repeat: no-repeat;
        background-size: 50px 30px;
        min-width: 50px;
        min-height: 30px;
        pointer-events: none;
        z-index: 9999;
    }

    .ingress-tooltip-body {
        margin: 8px;

        &__info {
            display: flex;
            background-color: var(--ingress-tooltip-info-background-color);
            border-radius: 12px;
            padding: 14px;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 14px;
            position: relative;
            color: var(--ingress-font-color);
        }
    }

    .ingress-tooltip-bold-text {
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
        color: var(--regular-text-color);
    }
</style>
