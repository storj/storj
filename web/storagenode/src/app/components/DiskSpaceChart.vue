// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="disk-space-chart__data-dimension">{{ chartDataDimension }}*h</p>
        <VChart
            id="disk-space-chart"
            :chart-data="chartData"
            :width="chartWidth"
            :height="chartHeight"
            :tooltip-constructor="diskSpaceTooltip"
            :key="chartKey"
        />
    </div>
</template>

<script lang="ts">
import { Component } from 'vue-property-decorator';

import BaseChart from '@/app/components/BaseChart.vue';

import { ChartData } from '@/app/types/chartData';
import { Tooltip, TooltipParams } from '@/app/types/tooltip';
import { ChartUtils } from '@/app/utils/chart';
import { formatBytes } from '@/app/utils/converter';
import { Stamp } from '@/storagenode/satellite';

/**
 * stores stamp data for disc space chart's tooltip
 */
class StampTooltip {
    public atRestTotal: string;
    public date: string;

    public constructor(stamp: Stamp) {
        this.atRestTotal = formatBytes(stamp.atRestTotal);
        this.date = stamp.intervalStart.toUTCString().slice(0, 16);
    }
}

@Component
export default class DiskSpaceChart extends BaseChart {
    private get chartBackgroundColor(): string {
        return this.isDarkMode ? '#4F97F7' : '#F2F6FC';
    }

    private get allStamps(): Stamp[] {
        return ChartUtils.populateEmptyStamps(this.$store.state.node.storageChartData);
    }

    public get chartDataDimension(): string {
        if (!this.$store.state.node.storageChartData.length) {
            return 'Bytes';
        }

        return ChartUtils.getChartDataDimension(this.allStamps.map((elem) => {
            return elem.atRestTotal;
        }));
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart();
        const chartBackgroundColor = this.chartBackgroundColor;
        const chartBorderColor = '#1F49A3';
        const chartBorderWidth = 1;

        if (this.allStamps.length) {
            data = ChartUtils.normalizeChartData(this.allStamps.map(elem => elem.atRestTotal));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public diskSpaceTooltip(tooltipModel): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'disk-space-chart', 'disk-space-tooltip',
            'disk-space-tooltip-arrow', 'disk-space-tooltip-point', this.tooltipMarkUp(tooltipModel),
            125, 89, 38, 24, 6, 4, `#1f49a3`);

        Tooltip.custom(tooltipParams);
    }

    private tooltipMarkUp(tooltipModel: any): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new StampTooltip(this.allStamps[dataIndex]);

        return `<div class='tooltip-body'>
                    <p class='tooltip-body__data'><b>${dataPoint.atRestTotal}*h</b></p>
                    <p class='tooltip-body__footer'>${dataPoint.date}</p>
                </div>`;
    }
}
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
            margin: 0 0 5px 31px !important;
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
