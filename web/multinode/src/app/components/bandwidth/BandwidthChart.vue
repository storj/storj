// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="bandwidth-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            id="bandwidth-chart"
            :key="chartKey"
            :chart-data="chartData"
            :width="chartWidth"
            :height="chartHeight"
            :tooltip-constructor="bandwidthTooltip"
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
 * stores bandwidth data for bandwidth chart's tooltip
 */
class BandwidthTooltip {
    public normalEgress: string;
    public normalIngress: string;
    public repairIngress: string;
    public repairEgress: string;
    public auditEgress: string;
    public date: string;

    public constructor(bandwidth: BandwidthRollup) {
        this.normalEgress = Size.toBase10String(bandwidth.egress.usage);
        this.normalIngress = Size.toBase10String(bandwidth.ingress.usage);
        this.repairIngress = Size.toBase10String(bandwidth.ingress.repair);
        this.repairEgress = Size.toBase10String(bandwidth.egress.repair);
        this.auditEgress = Size.toBase10String(bandwidth.egress.audit);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

// @vue/component
@Component({
    components: { VChart },
})
export default class BandwidthChart extends BaseChart {
    private get allBandwidth(): BandwidthRollup[] {
        return ChartUtils.populateEmptyBandwidth(this.$store.state.bandwidth.traffic.bandwidthDaily);
    }

    public get chartDataDimension(): string {
        if (!this.$store.state.bandwidth.traffic.bandwidthDaily.length) {
            return 'Bytes';
        }

        return ChartUtils.getChartDataDimension(this.allBandwidth.map((elem) => elem.egress.usage + elem.egress.repair + elem.egress.audit
                + elem.ingress.repair + elem.ingress.usage));
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart();
        const chartBackgroundColor = '#F2F6FC';
        const chartBorderColor = '#1F49A3';
        const chartBorderWidth = 1;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map(elem => elem.egress.usage + elem.egress.repair + elem.egress.audit
                    + elem.ingress.repair + elem.ingress.usage));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public bandwidthTooltip(tooltipModel: TooltipModel): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'bandwidth-tooltip',
            'bandwidth-tooltip-point', this.tooltipMarkUp(tooltipModel),
            285, 125, 6, 4, '#1f49a3');

        Tooltip.custom(tooltipParams);
    }

    private tooltipMarkUp(tooltipModel: TooltipModel): string {
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
            color: var(--v-header-base);
            margin: 0 0 5px 31px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #bandwidth-tooltip {
        background: var(--v-background2-base);
        border: 1px solid var(--v-border-base);
        min-width: 250px;
        min-height: 230px;
        font-size: 12px;
        border-radius: 14px;
        font-family: 'font_bold', sans-serif;
        color: var(--v-header-base);
        pointer-events: none;
        z-index: 9999;
    }

    #bandwidth-tooltip-point {
        z-index: 9999;
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
            border-radius: 12px;
            padding: 14px 17px 14px 14px;
            align-items: center;
            margin-bottom: 14px;
            position: relative;
            font-family: 'font_bold', sans-serif;

            .tooltip-bold-text {
                color: var(--v-primary-base);
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
        padding: 10px 0 16px;
        color: var(--v-header-base);
    }
</style>
