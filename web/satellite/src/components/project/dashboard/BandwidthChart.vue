// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VChart
        :key="chartKey"
        chart-id="bandwidth-chart"
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
    settledData: DataStamp[],
    allocatedData: DataStamp[],
    since: Date,
    before: Date,
    width: number,
    height: number,
    isVuetify?: boolean,
}>(), {
    settledData: () => [],
    allocatedData: () => [],
    since: () => new Date(),
    before: () => new Date(),
    width: 0,
    height: 0,
    isVuetify: false,
});

const chartKey = ref<number>(0);

/**
 * Returns formatted data to render chart.
 */
const chartData = computed((): ChartData => {
    const mainData: number[] = props.settledData.map(el => el.value);
    const secondaryData: number[] = props.allocatedData.map(el => el.value);
    const xAxisDateLabels: string[] = ChartUtils.daysDisplayedOnChart(props.since, props.before);

    return {
        labels: xAxisDateLabels,
        datasets: [{
            data: mainData,
            fill: true,
            backgroundColor: 'rgba(226, 220, 255, .3)',
            borderColor: '#7B61FF',
            pointHoverBackgroundColor: '#FFFFFF',
            pointBorderColor: '#7B61FF',
            pointHoverBorderWidth: 3,
            hoverRadius: 8,
            hitRadius: 3,
            pointRadius: 2,
            pointBorderWidth: 1,
            pointBackgroundColor: '#FFFFFF',
            order: 0,
        }, {
            data: secondaryData,
            fill: true,
            backgroundColor: 'rgba(226, 220, 255, .7)',
            borderColor: '#E2DCFF',
            pointHoverBackgroundColor: '#FFFFFF',
            pointBorderColor: '#E2DCFF',
            pointHoverBorderWidth: 3,
            hoverRadius: 8,
            hitRadius: 3,
            pointRadius: 2,
            pointBorderWidth: 1,
            pointBackgroundColor: '#FFFFFF',
            order: 1,
        }],
    };
});

/**
 * Used as constructor of custom tooltip.
 */
function tooltip(tooltipModel: TooltipModel<ChartType>): void {
    if (!tooltipModel.dataPoints) {
        const settledTooltip = Tooltip.createTooltip('settled-bandwidth-tooltip');
        const allocatedTooltip = Tooltip.createTooltip('allocated-bandwidth-tooltip');
        Tooltip.remove(settledTooltip);
        Tooltip.remove(allocatedTooltip);

        return;
    }

    tooltipModel.dataPoints.forEach(p => {
        let tooltipParams: TooltipParams;
        if (p.datasetIndex === 0) {
            tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'settled-bandwidth-tooltip',
                settledTooltipMarkUp(tooltipModel), -20, props.isVuetify ? 68 : 78);
        } else {
            tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'allocated-bandwidth-tooltip',
                allocatedTooltipMarkUp(tooltipModel), 95, props.isVuetify ? 68 : 78);
        }

        Tooltip.custom(tooltipParams);
    });
}

/**
 * Returns allocated bandwidth tooltip's html mark up.
 */
function allocatedTooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new ChartTooltipData(props.allocatedData[dataIndex]);

    return `<div class='allocated-tooltip'>
                <p class='settled-tooltip__title'>Allocated</p>
                <p class='allocated-tooltip__value'>${dataPoint.date}<b class='allocated-tooltip__value__bold'> / ${dataPoint.value}</b></p>
                <div class='allocated-tooltip__arrow'></div>
            </div>`;
}

/**
 * Returns settled bandwidth tooltip's html mark up.
 */
function settledTooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new ChartTooltipData(props.settledData[dataIndex]);

    return `<div class='settled-tooltip'>
                <div class='settled-tooltip__arrow'></div>
                <p class='settled-tooltip__title'>Settled</p>
                <p class='settled-tooltip__value'>${dataPoint.date}<b class='settled-tooltip__value__bold'> / ${dataPoint.value}</b></p>
            </div>`;
}

watch(() => props.width, () => {
    chartKey.value += 1;
});
</script>

<style lang="scss">
    .settled-tooltip,
    .allocated-tooltip {
        margin: 8px;
        position: relative;
        border-radius: 14.5px;
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        width: 120px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 17px;
            color: #fff;
            align-self: flex-start;
        }

        &__value {
            font-size: 14px;
            line-height: 26px;
            text-align: center;
            color: #fff;
            white-space: nowrap;
            align-self: flex-start;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }
        }

        &__arrow {
            width: 12px;
            height: 12px;
        }
    }

    .settled-tooltip {
        background-color: var(--c-purple-3);
        padding: 4px 10px 8px;

        &__arrow {
            margin: -12px 0 4px;
            border-radius: 0 0 0 8px;
            transform: scale(1, 0.85) translate(0, 20%) rotate(-45deg);
            background-color: var(--c-purple-3);
        }
    }

    .allocated-tooltip {
        background-color: var(--c-purple-2);
        padding: 8px 10px 0;

        &__arrow {
            margin-bottom: -4px;
            border-radius: 8px 0 0;
            transform: scale(1, 0.85) translate(0, 20%) rotate(45deg);
            background-color: var(--c-purple-2);
        }
    }
</style>
