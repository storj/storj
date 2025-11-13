// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <p class="egress-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            :key="chartKey"
            chart-id="egress-chart"
            :chart-data="chartData"
            :width="width"
            :height="height"
            :tooltip-constructor="egressTooltip"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ChartData, ChartType, TooltipModel } from 'chart.js';

import { Tooltip, TooltipParams } from '@/app/types/chart';
import { ChartUtils } from '@/app/utils/chart';
import { Size } from '@/private/memory/size';
import { EgressUsed } from '@/storagenode/sno/sno';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import VChart from '@/app/components/VChart.vue';

/**
 * stores egress data for egress bandwidth chart's tooltip
 */
class EgressTooltip {
    public normalEgress: string;
    public repairEgress: string;
    public auditEgress: string;
    public date: string;

    public constructor(bandwidth: EgressUsed) {
        this.normalEgress = Size.toBase10String(bandwidth.egress.usage);
        this.repairEgress = Size.toBase10String(bandwidth.egress.repair);
        this.auditEgress = Size.toBase10String(bandwidth.egress.audit);
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
    return props.isDarkMode ? '#4FC895' : '#edf9f4';
});

const allBandwidth = computed<EgressUsed[]>(() => {
    return ChartUtils.populateEmptyBandwidth(nodeStore.state.egressChartData, EgressUsed);
});

const chartDataDimension = computed<string>(() => {
    if (!nodeStore.state.egressChartData.length) {
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
                borderColor: '#48a77f',
                borderWidth: 1,
                pointHoverBorderWidth: 3,
                hoverRadius: 8,
                hitRadius: 8,
                pointRadius: 4,
                pointBorderWidth: 1,
                data: data,
            },
        ],
    };
});

function rebuildChart(): void {
    chartKey.value += 1;
}

function egressTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'egress-chart', 'egress-tooltip',
        tooltipMarkUp(tooltipModel), 255, 94);

    Tooltip.custom(tooltipParams);
}

function tooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new EgressTooltip(allBandwidth.value[dataIndex]);

    return `<div class='egress-tooltip-body'>
                <div class='egress-tooltip-body__info'>
                    <p>USAGE</p>
                    <b class="egress-tooltip-bold-text">${dataPoint.normalEgress}</b>
                </div>
                <div class='egress-tooltip-body__info'>
                    <p>REPAIR</p>
                    <b class="egress-tooltip-bold-text">${dataPoint.repairEgress}</b>
                </div>
                <div class='egress-tooltip-body__info'>
                    <p>AUDIT</p>
                    <b class="egress-tooltip-bold-text">${dataPoint.auditEgress}</b>
                </div>
            </div>
            <div class='egress-tooltip-footer'>
                <p>${dataPoint.date}</p>
            </div>`;
}

watch([() => props.isDarkMode, chartData, () => props.width], rebuildChart);
</script>

<style lang="scss">
    .egress-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--regular-text-color);
            margin: 0 0 5px 3px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #egress-tooltip {
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

    #egress-tooltip-arrow {
        background-image: var(--tooltip-arrow-path);
        background-repeat: no-repeat;
        background-size: 50px 30px;
        min-width: 50px;
        min-height: 30px;
        pointer-events: none;
        z-index: 9999;
    }

    .egress-tooltip-body {
        margin: 8px;

        &__info {
            display: flex;
            background-color: var(--egress-tooltip-info-background-color);
            border-radius: 12px;
            padding: 14px;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 14px;
            position: relative;
            color: var(--egress-font-color);
        }
    }

    .egress-tooltip-bold-text {
        font-size: 14px;
    }

    .egress-tooltip-footer {
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
