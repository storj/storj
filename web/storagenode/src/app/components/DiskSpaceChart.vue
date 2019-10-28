// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="disk-space-chart__data-dimension">{{chartDataDimension}}*h</p>
        <VChart
            id="disk-space-chart"
            :chart-data="chartData"
            :width="400"
            :height="240"
            :tooltip-constructor="diskSpaceTooltip"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VChart from '@/app/components/VChart.vue';

import { ChartData } from '@/app/types/chartData';
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
        this.date = stamp.intervalStart.toUTCString();
    }
}

@Component ({
    components: {
        VChart,
    },
})
export default class DiskSpaceChart extends Vue {
    private readonly TOOLTIP_MARGIN: number = 20;
    private readonly TOOLTIP_OPACITY: string = '1';
    private readonly TOOLTIP_POSITION: string = 'absolute';

    private get allStamps(): Stamp[] {
        return ChartUtils.populateEmptyStamps(this.$store.state.node.storageChartData);
    }

    public get chartDataDimension(): string {
        if (!this.allStamps.length) {
            return '';
        }

        return ChartUtils.getChartDataDimension(this.allStamps.map((elem) => {
            return elem.atRestTotal;
        }));
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart();
        const chartBackgroundColor = '#F2F6FC';
        const chartBorderColor = '#1F49A3';
        const chartBorderWidth = 2;

        if (this.allStamps.length) {
            data = ChartUtils.normalizeChartData(this.allStamps.map(elem => elem.atRestTotal));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public diskSpaceTooltip(tooltipModel): void {
        // Tooltip Element
        let tooltipEl = document.getElementById('disk-space-tooltip');
        // Create element on first render
        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.id = 'disk-space-tooltip';
            document.body.appendChild(tooltipEl);
        }

        // Hide if no tooltip
        if (!tooltipModel.opacity) {
            document.body.removeChild(tooltipEl);

            return;
        }

        // Set Text
        if (tooltipModel.body) {
            const dataIndex = tooltipModel.dataPoints[0].index;
            const dataPoint = new StampTooltip(this.allStamps[dataIndex]);

            tooltipEl.innerHTML = `<div class='tooltip-body'>
                                       <p class='tooltip-body__data'><b>${dataPoint.atRestTotal}*h</b></p>
                                       <p class='tooltip-body__footer'>${dataPoint.date}</p>
                                   </div>`;
        }

        const diskSpaceChart = document.getElementById('disk-space-chart');

        if (diskSpaceChart) {
            const position = diskSpaceChart.getBoundingClientRect();
            tooltipEl.style.opacity = this.TOOLTIP_OPACITY;
            tooltipEl.style.position = this.TOOLTIP_POSITION;
            tooltipEl.style.right = position.left + window.pageXOffset - tooltipModel.caretX - this.TOOLTIP_MARGIN + 'px';
            tooltipEl.style.top = position.top + window.pageYOffset + tooltipModel.caretY + 'px';
        }

        return;
    }
}
</script>

<style lang="scss">
    p {
        margin: 0;
    }

    .disk-space-chart {

        &__data-dimension {
            font-size: 13px;
            color: #586c86;
            margin: 0 0 5px 30px;
            font-family: 'font_medium', sans-serif;
        }
    }

    #disk-space-tooltip {
        background-color: #fff;
        width: auto;
        font-size: 12px;
        border-radius: 8px;
        box-shadow: 0 2px 10px #d2d6de;
        color: #535f77;
        padding: 6px;
        pointer-events: none;
    }

    .tooltip-body {

        &__data {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 11px 44px 11px 44px;
            font-size: 14px;
        }

        &__footer {
            font-size: 12px;
            width: auto;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 10px 0 16px 0;
        }
    }
</style>
