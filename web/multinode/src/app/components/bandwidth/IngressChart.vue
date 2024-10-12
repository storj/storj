// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="ingress-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            id="ingress-chart"
            :key="chartKey"
            :chart-data="chartData"
            :width="chartWidth"
            :height="chartHeight"
            :tooltip-constructor="ingressTooltip"
        />
    </div>
</template>

<script lang="ts">
import { Component } from 'vue-property-decorator';

import { ChartData, Tooltip, TooltipParams, TooltipModel } from '@/app/types/chart';
import { Chart as ChartUtils } from '@/app/utils/chart';
import { BandwidthRollup } from '@/bandwidth';
import { Size } from '@/private/memory/size';

import VChart from '@/app/components/common/VChart.vue';
import BaseChart from '@/app/components/common/BaseChart.vue';

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

// @vue/component
@Component({
    components: { VChart },
})
export default class IngressChart extends BaseChart {
    private get allBandwidth(): BandwidthRollup[] {
        return ChartUtils.populateEmptyBandwidth(this.$store.state.bandwidth.traffic.bandwidthDaily);
    }

    public get chartDataDimension(): string {
        if (!this.$store.state.bandwidth.traffic.bandwidthDaily.length) {
            return 'Bytes';
        }

        return ChartUtils.getChartDataDimension(this.allBandwidth.map((elem) => elem.ingress.repair + elem.ingress.usage));
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart();
        const chartBackgroundColor = this.$vuetify.theme.dark ? '#f7e8cb' : '#fff4df';
        const chartBorderColor = this.$vuetify.theme.dark ? '#ffad12' : '#e1a128';
        const chartBorderWidth = 1;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map(elem => elem.ingress.repair + elem.ingress.usage));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public ingressTooltip(tooltipModel: TooltipModel): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'ingress-chart', 'ingress-tooltip',
            'ingress-tooltip-point', this.tooltipMarkUp(tooltipModel),
            185, 94, 6, 4, '#e1a128');

        Tooltip.custom(tooltipParams);
    }

    private tooltipMarkUp(tooltipModel: TooltipModel): string {
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
