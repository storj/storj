// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <p class="disk-space-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            :key="chartKey"
            chart-id="disk-space-chart"
            :chart-data="chartData"
            :width="width"
            :height="height"
            :tooltip-constructor="diskSpaceTooltip"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ChartData, ChartType, TooltipModel } from 'chart.js';

import { Tooltip, TooltipParams } from '@/app/types/chart';
import { ChartUtils } from '@/app/utils/chart';
import { Size } from '@/private/memory/size';
import { Stamp } from '@/storagenode/sno/sno';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import VChart from '@/app/components/VChart.vue';

/**
 * stores stamp data for disc space chart's tooltip
 */
class StampTooltip {
    public atRestTotal: string;
    public atRestTotalBytes: string;
    public date: string;

    public constructor(stamp: Stamp) {
        this.atRestTotal = Size.toBase10String(stamp.atRestTotal);
        this.atRestTotalBytes = Size.toBase10String(stamp.atRestTotalBytes);
        this.date = stamp.intervalStart.toUTCString().slice(0, 16);
    }
}

const nodeStore = useNodeStore();

const props = defineProps<{
    width: number;
    height: number;
    isDarkMode: boolean;
}>();

const chartKey = ref<number>(0);

const allStamps = computed<Stamp[]>(() => {
    return ChartUtils.populateEmptyStamps(nodeStore.state.storageChartData);
});

const chartBackgroundColor = computed<string>(() => {
    return props.isDarkMode ? '#4F97F7' : '#F2F6FC';
});

const chartDataDimension = computed<string>(() => {
    if (!nodeStore.state.storageChartData.length) {
        return 'Bytes';
    }

    return ChartUtils.getChartDataDimension(allStamps.value.map((elem) => {
        return elem.atRestTotalBytes;
    }));
});

const chartData = computed<ChartData>(() => {
    let data: number[] = [0];
    if (allStamps.value.length) {
        data = ChartUtils.normalizeChartData(allStamps.value.map(elem => elem.atRestTotalBytes));
    }

    return {
        labels: ChartUtils.daysDisplayedOnChart(),
        datasets: [
            {
                fill: true,
                backgroundColor: chartBackgroundColor.value,
                borderColor: '#1F49A3',
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

function diskSpaceTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'disk-space-chart', 'disk-space-tooltip',
        tooltipMarkUp(tooltipModel), 125, 89);

    Tooltip.custom(tooltipParams);
}

function tooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new StampTooltip(allStamps.value[dataIndex]);

    return `<div class='tooltip-body'>
                <p class='tooltip-body__data'><b>${dataPoint.atRestTotalBytes}</b></p>
                <p class='tooltip-body__footer'>${dataPoint.date}</p>
            </div>`;
}

watch([() => props.isDarkMode, chartData, () => props.width], rebuildChart);
</script>

<style lang="scss">
    p {
        margin: 0;
    }

    .disk-space-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--regular-text-color);
            margin: 0 0 5px 3px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #disk-space-tooltip {
        background-image: var(--tooltip-background-path);
        background-repeat: no-repeat;
        background-size: cover;
        width: 180px;
        height: 90px;
        font-size: 12px;
        border-radius: 14px;
        color: var(--regular-text-color);
        pointer-events: none;
        z-index: 9999;
    }

    #disk-space-tooltip-arrow {
        background-image: var(--tooltip-arrow-path);
        background-repeat: no-repeat;
        background-size: 50px 30px;
        min-width: 50px;
        min-height: 30px;
        pointer-events: none;
    }

    .tooltip-body {

        &__data {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 11px 44px;
            font-size: 14px;
        }

        &__footer {
            font-size: 12px;
            width: auto;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 10px 0;
        }
    }
</style>
