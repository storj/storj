// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="ingress-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            id="ingress-chart"
            :chart-data="chartData"
            :width="chartWidth"
            :height="chartHeight"
            :tooltip-constructor="ingressTooltip"
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
import { IngressUsed } from '@/storagenode/satellite';

/**
 * stores ingress data for ingress bandwidth chart's tooltip
 */
class IngressTooltip {
    public normalIngress: string;
    public repairIngress: string;
    public date: string;

    public constructor(bandwidth: IngressUsed) {
        this.normalIngress = formatBytes(bandwidth.ingress.usage);
        this.repairIngress = formatBytes(bandwidth.ingress.repair);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

@Component
export default class IngressChart extends BaseChart {
    private get chartBackgroundColor(): string {
        return this.isDarkMode ? '#E1A128' : '#fff4df';
    }

    private get allBandwidth(): IngressUsed[] {
        return ChartUtils.populateEmptyBandwidth(this.$store.state.node.ingressChartData);
    }

    public get chartDataDimension(): string {
        if (!this.$store.state.node.ingressChartData.length) {
            return 'Bytes';
        }

        return ChartUtils.getChartDataDimension(this.allBandwidth.map((elem) => {
            return elem.summary();
        }));
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart();
        const chartBackgroundColor = this.chartBackgroundColor;
        const chartBorderColor = '#e1a128';
        const chartBorderWidth = 1;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map(elem => elem.summary()));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public ingressTooltip(tooltipModel): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'ingress-chart', 'ingress-tooltip',
            'ingress-tooltip-arrow', 'ingress-tooltip-point', this.tooltipMarkUp(tooltipModel),
            205, 94, 35, 24, 6, 4, `#e1a128`);

        Tooltip.custom(tooltipParams);
    }

    private tooltipMarkUp(tooltipModel: any): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new IngressTooltip(this.allBandwidth[dataIndex]);

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
}
</script>

<style lang="scss">
    .ingress-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--regular-text-color);
            margin: 0 0 5px 31px !important;
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
    }

    #ingress-tooltip-arrow {
        background-image: var(--tooltip-arrow-path);
        background-repeat: no-repeat;
        background-size: 50px 30px;
        min-width: 50px;
        min-height: 30px;
        pointer-events: none;
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
        padding: 10px 0 16px 0;
        color: var(--regular-text-color);
    }
</style>
