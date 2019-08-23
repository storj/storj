// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <Chart
            id="disk-space-chart"
            :chartData="chartData"
            :width="400"
            :height="200"
            :tooltipConstructor="diskSpaceTooltip" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Chart from '@/app/components/Chart.vue';
import { ChartUtils } from '@/app/utils/chart';
import { ChartFormatter } from '@/app/utils/chartModule';
import { formatBytes } from '@/app/utils/converter';
import { Stamp } from '@/storagenode/satellite';

class StampTooltip {
    public atRestTotal: string;
    public timestamp: string;

    public constructor(stamp: Stamp) {
        this.atRestTotal = formatBytes(stamp.atRestTotal);
        this.timestamp = stamp.timestamp.toLocaleString();
    }
}

@Component ({
    components: {
        Chart,
    },
})
export default class DiskSpaceChart extends Vue {
    private get allStamps(): Stamp[] {
        const stamps: Stamp[] = ChartFormatter.populateEmptyStamps(this.$store.state.node.storageChartData);

        return stamps;
    }

    public get chartData(): any {
        let data: number[] = ChartUtils.normalizeArray(this.allStamps.map(elem => elem.atRestTotal));

        const tillDate = new Date();
        tillDate.setDate(tillDate.getDate());

        const result = {
            labels: ChartUtils.xAxeOptions(tillDate),
            datasets: [{
                backgroundColor: '#F2F6FC',
                borderColor: '#1F49A3',
                borderWidth: 2,
                data,
            }],
        };

        // TODO: create needed type
        return result;
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
        if (tooltipModel.opacity === 0) {
            document.body.removeChild(tooltipEl);

            return;
        }

        // Set Text
        if (tooltipModel.body) {
            const index = tooltipModel.dataPoints[0].index;
            const point = new StampTooltip(this.allStamps[index]);

            tooltipEl.innerHTML = `<div class='tooltip-body'>
                                       <p class='tooltip-body__data'><b>${point.atRestTotal}</b></p>
                                       <p class='tooltip-body__footer'>${point.timestamp}</p>
                                   </div>`;
        }

        let diskSpaceChart = document.getElementById('disk-space-chart');

        if (diskSpaceChart) {
            let position = diskSpaceChart.getBoundingClientRect();
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
        font-family: 'font_regular';
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
