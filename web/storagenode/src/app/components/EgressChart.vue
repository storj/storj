// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="egress-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            id="egress-chart"
            :chart-data="chartData"
            :width="chartWidth"
            :height="chartHeight"
            :tooltip-constructor="egressTooltip"
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
import { EgressUsed } from '@/storagenode/satellite';

/**
 * stores egress data for egress bandwidth chart's tooltip
 */
class EgressTooltip {
    public normalEgress: string;
    public repairEgress: string;
    public auditEgress: string;
    public date: string;

    public constructor(bandwidth: EgressUsed) {
        this.normalEgress = formatBytes(bandwidth.egress.usage);
        this.repairEgress = formatBytes(bandwidth.egress.repair);
        this.auditEgress = formatBytes(bandwidth.egress.audit);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

@Component
export default class EgressChart extends BaseChart {
    private get chartBackgroundColor(): string {
        return this.isDarkMode ? '#4FC895' : '#edf9f4';
    }

    private get allBandwidth(): EgressUsed[] {
        return ChartUtils.populateEmptyBandwidth(this.$store.state.node.egressChartData);
    }

    public get chartDataDimension(): string {
        if (!this.$store.state.node.egressChartData.length) {
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
        const chartBorderColor = '#48a77f';
        const chartBorderWidth = 1;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map(elem => elem.summary()));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public egressTooltip(tooltipModel): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'egress-chart', 'egress-tooltip',
            'egress-tooltip-arrow', 'egress-tooltip-point', this.tooltipMarkUp(tooltipModel),
            255, 94, 35, 24, 6, 4, `#48a77f`);

        Tooltip.custom(tooltipParams);
    }

    private tooltipMarkUp(tooltipModel: any): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new EgressTooltip(this.allBandwidth[dataIndex]);

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
}
</script>

<style lang="scss">
    .egress-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--regular-text-color);
            margin: 0 0 5px 31px !important;
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
    }

    #egress-tooltip-arrow {
        background-image: var(--tooltip-arrow-path);
        background-repeat: no-repeat;
        background-size: 50px 30px;
        min-width: 50px;
        min-height: 30px;
        pointer-events: none;
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
        padding: 10px 0 16px 0;
        color: var(--regular-text-color);
    }
</style>
