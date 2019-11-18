// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="egress-chart__data-dimension">{{chartDataDimension}}</p>
        <VChart
            id="egress-chart"
            :chart-data="chartData"
            :width="400"
            :height="240"
            :tooltip-constructor="egressBandwidthTooltip"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VChart from '@/app/components/VChart.vue';

import { ChartData } from '@/app/types/chartData';
import { ChartUtils } from '@/app/utils/chart';
import { formatBytes } from '@/app/utils/converter';
import { EgressUsed } from '@/storagenode/satellite';

/**
 * stores egress bandwidth data for egress bandwidth chart's tooltip
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

@Component ({
    components: {
        VChart,
    },
})
export default class EgressChart extends Vue {
    private readonly TOOLTIP_OPACITY: string = '1';
    private readonly TOOLTIP_POSITION: string = 'absolute';

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
        const chartBackgroundColor = '#edf9f4';
        const chartBorderColor = '#48a77f';
        const chartBorderWidth = 2;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map((elem) => {
                return elem.summary();
            }));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public egressBandwidthTooltip(tooltipModel): void {
        // Tooltip Element
        let tooltipEl = document.getElementById('egress-tooltip');
        // Create element on first render
        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.id = 'egress-tooltip';
            document.body.appendChild(tooltipEl);
        }

        // Tooltip Arrow
        let tooltipArrow = document.getElementById('egress-tooltip-arrow');
        // Create element on first render
        if (!tooltipArrow) {
            tooltipArrow = document.createElement('div');
            tooltipArrow.id = 'egress-tooltip-arrow';
            document.body.appendChild(tooltipArrow);
        }

        // Hide if no tooltip
        if (!tooltipModel.opacity) {
            document.body.removeChild(tooltipEl);
            document.body.removeChild(tooltipArrow);

            return;
        }

        // Set Text
        if (tooltipModel.body) {
            const dataIndex = tooltipModel.dataPoints[0].index;
            const dataPoint = new EgressTooltip(this.allBandwidth[dataIndex]);

            tooltipEl.innerHTML = `<div class='egress-tooltip-body'>
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

        // `this` will be the overall tooltip
        const bandwidthChart = document.getElementById('egress-chart');
        if (bandwidthChart) {
            const position = bandwidthChart.getBoundingClientRect();
            tooltipEl.style.opacity = this.TOOLTIP_OPACITY;
            tooltipEl.style.position = this.TOOLTIP_POSITION;
            tooltipEl.style.left = position.left + tooltipModel.caretX - 94 + 'px';
            tooltipEl.style.bottom = position.bottom + window.pageYOffset - tooltipModel.caretY - 83 + 'px';

            tooltipArrow.style.opacity = this.TOOLTIP_OPACITY;
            tooltipArrow.style.position = this.TOOLTIP_POSITION;
            tooltipArrow.style.left = position.left + tooltipModel.caretX - 24 + 'px';
            tooltipArrow.style.bottom = position.bottom + window.pageYOffset - tooltipModel.caretY - 103 + 'px';
        }

        return;
    }
}
</script>

<style lang="scss">
    p {
        margin: 0;
    }

    .egress-chart {

        &__data-dimension {
            font-size: 13px;
            color: #586c86;
            margin: 0 0 5px 31px;
            font-family: 'font_medium', sans-serif;
        }
    }

    #egress-tooltip {
        background-image: url('../../../static/images/tooltipBack.png');
        background-repeat: no-repeat;
        background-size: cover;
        min-width: 190px;
        min-height: 170px;
        font-size: 12px;
        border-radius: 14px;
        box-shadow: 0 2px 10px #d2d6de;
        color: #535f77;
        pointer-events: none;
    }

    #egress-tooltip-arrow {
        background-image: url('../../../static/images/tooltipArrow.png');
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
            background-color: rgba(211, 242, 204, 0.3);
            border-radius: 12px;
            padding: 14px;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 14px;
            position: relative;
            color: #2e5f46;
        }
    }

    .egress-tooltip-bold-text {
        font-family: 'font_bold', sans-serif;
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
        color: rgba(83, 95, 119, 0.44);
    }
</style>
