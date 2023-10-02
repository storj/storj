// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VChart
        :key="chartKey"
        chart-id="storage-chart"
        :chart-data="chartData"
        :width="width"
        :height="height"
        :tooltip-constructor="tooltip"
    />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ChartType, TooltipModel, ChartData } from 'chart.js';

import { Tooltip, TooltipParams, ChartTooltipData } from '@/types/chart';
import { DataStamp } from '@/types/projects';
import { ChartUtils } from '@/utils/chart';

import VChart from '@/components/common/VChart.vue';

const props = withDefaults(defineProps<{
    data: DataStamp[],
    since: Date,
    before: Date,
    width: number,
    height: number,
}>(), {
    data: () => [],
    since: () => new Date(),
    before: () => new Date(),
    width: 0,
    height: 0,
});

const chartKey = ref<number>(0);

/**
 * Returns formatted data to render chart.
 */
const chartData = computed((): ChartData => {
    const data: number[] = props.data.map(el => el.value);
    const xAxisDateLabels: string[] = ChartUtils.daysDisplayedOnChart(props.since, props.before);

    return {
        labels: xAxisDateLabels,
        datasets: [{
            data,
            fill: true,
            backgroundColor: '#929fb110',
            borderColor: '#929fb1',
            pointHoverBackgroundColor: '#FFFFFF',
            pointBorderColor: '#929fb1',
            pointHoverBorderWidth: 3,
            hoverRadius: 8,
            hitRadius: 3,
            pointRadius: 2,
            pointBorderWidth: 1,
            pointBackgroundColor: '#FFFFFF',
        }],
    };
});

/**
 * Used as constructor of custom tooltip.
 */
function tooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'storage-chart', 'storage-tooltip',
        tooltipMarkUp(tooltipModel), 76, 81);

    Tooltip.custom(tooltipParams);
}

/**
 * Returns tooltip's html mark up.
 */
function tooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new ChartTooltipData(props.data[dataIndex]);

    return `<div class='tooltip'>
                <p class='tooltip__value'>${dataPoint.date}<b class='tooltip__value__bold'> / ${dataPoint.value}</b></p>
                <div class='tooltip__arrow' />
            </div>`;
}

watch(() => props.width, () => {
    chartKey.value += 1;
});
</script>

<style lang="scss">
    .tooltip {
        margin: 8px;
        position: relative;
        border-radius: 100px;
        padding-top: 8px;
        width: 145px;
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        background-color: #929fb1;

        &__value {
            font-size: 14px;
            line-height: 26px;
            text-align: center;
            color: var(--c-white);
            white-space: nowrap;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }
        }

        &__arrow {
            width: 12px;
            height: 12px;
            border-radius: 8px 0 0;
            transform: scale(1, 0.85) translate(0, 20%) rotate(45deg);
            margin-bottom: -4px;
            background-color: #929fb1;
        }
    }
</style>
