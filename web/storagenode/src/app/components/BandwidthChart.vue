// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="bandwidth-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            id="bandwidth-chart"
            :chart-data="chartData"
            :width="chartWidth"
            :height="chartHeight"
            :tooltip-constructor="bandwidthTooltip"
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
import { BandwidthUsed } from '@/storagenode/satellite';

/**
 * stores bandwidth data for bandwidth chart's tooltip
 */
class BandwidthTooltip {
    public normalEgress: string;
    public normalIngress: string;
    public repairIngress: string;
    public repairEgress: string;
    public auditEgress: string;
    public date: string;

    public constructor(bandwidth: BandwidthUsed) {
        this.normalEgress = formatBytes(bandwidth.egress.usage);
        this.normalIngress = formatBytes(bandwidth.ingress.usage);
        this.repairIngress = formatBytes(bandwidth.ingress.repair);
        this.repairEgress = formatBytes(bandwidth.egress.repair);
        this.auditEgress = formatBytes(bandwidth.egress.audit);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

@Component
export default class BandwidthChart extends BaseChart {
    private get chartBackgroundColor(): string {
        return this.isDarkMode ? '#4F97F7' : '#F2F6FC';
    }

    private get allBandwidth(): BandwidthUsed[] {
        return ChartUtils.populateEmptyBandwidth(this.$store.state.node.bandwidthChartData);
    }

    public get chartDataDimension(): string {
        if (!this.$store.state.node.bandwidthChartData.length) {
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
        const chartBorderColor = '#1F49A3';
        const chartBorderWidth = 1;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map(elem => elem.summary()));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public bandwidthTooltip(tooltipModel: any): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'bandwidth-tooltip',
            'bandwidth-tooltip-arrow', 'bandwidth-tooltip-point', this.tooltipMarkUp(tooltipModel),
            303, 125, 35, 24, 6, 4, `#1f49a3`);

        Tooltip.custom(tooltipParams);
    }

    private tooltipMarkUp(tooltipModel: any): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new BandwidthTooltip(this.allBandwidth[dataIndex]);

        return `<div class='tooltip-header'>
                    <p>EGRESS</p>
                    <p class='tooltip-header__ingress'>INGRESS</p>
                </div>
                <div class='tooltip-body'>
                    <div class='tooltip-body__info'>
                        <p>USAGE</p>
                        <p class='tooltip-body__info__egress-value'><b class="tooltip-bold-text">${dataPoint.normalEgress}</b></p>
                        <p class='tooltip-body__info__ingress-value'><b class="tooltip-bold-text">${dataPoint.normalIngress}</b></p>
                    </div>
                    <div class='tooltip-body__info'>
                        <p>REPAIR</p>
                        <p class='tooltip-body__info__egress-value'><b class="tooltip-bold-text">${dataPoint.repairEgress}</b></p>
                        <p class='tooltip-body__info__ingress-value'><b class="tooltip-bold-text">${dataPoint.repairIngress}</b></p>
                    </div>
                    <div class='tooltip-body__info'>
                        <p>AUDIT</p>
                        <p class='tooltip-body__info__egress-value'><b class="tooltip-bold-text">${dataPoint.auditEgress}</b></p>
                    </div>
                </div>
                <div class='tooltip-footer'>
                    <p>${dataPoint.date}</p>
                </div>`;
    }
}
</script>

<style lang="scss">
    p {
        margin: 0;
    }

    .bandwidth-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--regular-text-color);
            margin: 0 0 5px 31px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #bandwidth-tooltip {
        background-image: var(--tooltip-background-path);
        background-repeat: no-repeat;
        background-size: cover;
        min-width: 250px;
        min-height: 230px;
        font-size: 12px;
        border-radius: 14px;
        color: var(--regular-text-color);
        pointer-events: none;
    }

    #bandwidth-tooltip-arrow {
        background-image: var(--tooltip-arrow-path);
        background-repeat: no-repeat;
        background-size: 50px 30px;
        min-width: 50px;
        min-height: 30px;
        pointer-events: none;
    }

    .tooltip-header {
        display: flex;
        padding: 10px 0 0 92px;
        line-height: 40px;

        &__ingress {
            margin-left: 29px;
        }
    }

    .tooltip-body {
        margin: 8px;

        &__info {
            display: flex;
            background-color: var(--block-background-color);
            border-radius: 12px;
            padding: 14px 17px 14px 14px;
            align-items: center;
            margin-bottom: 14px;
            position: relative;

            .tooltip-bold-text {
                font-size: 14px;
            }

            &__egress-value {
                position: absolute;
                left: 83px;
            }

            &__ingress-value {
                position: absolute;
                left: 158px;
            }
        }
    }

    .tooltip-footer {
        font-size: 12px;
        width: auto;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 10px 0 16px 0;
        color: var(--regular-text-color);
    }
</style>
