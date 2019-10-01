// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <VChart
            id="disk-space-chart"
            :chart-data="chartData"
            :width="400"
            :height="200"
            :tooltip-constructor="diskSpaceTooltip" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VChart from '@/app/components/VChart.vue';
import { ChartData } from '@/app/types/chartData';
import { ChartUtils } from '@/app/utils/chartUtils';
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
    private get allStamps(): Stamp[] {
        return ChartUtils.populateEmptyStamps(this.$store.state.node.storageChartData);
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart(new Date());
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
            tooltipEl.style.opacity = '1';
            tooltipEl.style.position = 'absolute';
            tooltipEl.style.right = position.left + window.pageXOffset - tooltipModel.caretX - 20 + 'px';
            tooltipEl.style.top = position.top + window.pageYOffset + tooltipModel.caretY + 'px';
        }

        return;
    }
}
</script>

<style lang="scss">
    #disk-space-tooltip {
        background-color: #FFFFFF;
        width: auto;
        font-size: 12px;
        border-radius: 8px;
        box-shadow: 0 2px 10px #D2D6DE;
        color: #535F77;
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
