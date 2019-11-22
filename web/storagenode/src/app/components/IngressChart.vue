// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <p class="ingress-chart__data-dimension">{{chartDataDimension}}</p>
        <VChart
            id="ingress-chart"
            :chart-data="chartData"
            :width="400"
            :height="240"
            :tooltip-constructor="ingressTooltip"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VChart from '@/app/components/VChart.vue';

import { ChartData } from '@/app/types/chartData';
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

@Component ({
    components: {
        VChart,
    },
})
export default class IngressChart extends Vue {
    private readonly TOOLTIP_OPACITY: string = '1';
    private readonly TOOLTIP_POSITION: string = 'absolute';

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
        const chartBackgroundColor = '#fff4df';
        const chartBorderColor = '#e1a128';
        const chartBorderWidth = 2;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map(elem => elem.summary()));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public ingressTooltip(tooltipModel): void {
        // Tooltip Element
        let tooltipEl = document.getElementById('ingress-tooltip');
        // Create element on first render
        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.id = 'ingress-tooltip';
            document.body.appendChild(tooltipEl);
        }

        // Tooltip Arrow
        let tooltipArrow = document.getElementById('ingress-tooltip-arrow');
        // Create element on first render
        if (!tooltipArrow) {
            tooltipArrow = document.createElement('div');
            tooltipArrow.id = 'ingress-tooltip-arrow';
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
            const dataPoint = new IngressTooltip(this.allBandwidth[dataIndex]);

            tooltipEl.innerHTML = `<div class='ingress-tooltip-body'>
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

        const ingressChart = document.getElementById('ingress-chart');
        if (!ingressChart) {
            return;
        }

        // `this` will be the overall tooltip.
        const position = ingressChart.getBoundingClientRect();
        tooltipEl.style.opacity = this.TOOLTIP_OPACITY;
        tooltipEl.style.position = this.TOOLTIP_POSITION;
        tooltipEl.style.left = `${position.left + tooltipModel.caretX - 94}px`;
        tooltipEl.style.bottom = `${position.bottom + window.pageYOffset - tooltipModel.caretY + 150}px`;

        tooltipArrow.style.opacity = this.TOOLTIP_OPACITY;
        tooltipArrow.style.position = this.TOOLTIP_POSITION;
        tooltipArrow.style.left = `${position.left + tooltipModel.caretX - 24}px`;
        tooltipArrow.style.bottom = `${position.bottom + window.pageYOffset - tooltipModel.caretY + 125}px`;
    }
}
</script>

<style lang="scss">
    p {
        margin: 0;
    }

    .ingress-chart {

        &__data-dimension {
            font-size: 13px;
            color: #586c86;
            margin: 0 0 5px 31px;
            font-family: 'font_medium', sans-serif;
        }
    }

    #ingress-tooltip {
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

    #ingress-tooltip-arrow {
        background-image: url('../../../static/images/tooltipArrow.png');
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
            background-color: rgba(254, 238, 215, 0.3);
            border-radius: 12px;
            padding: 14px;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 14px;
            position: relative;
            color: #6e4f15;
        }
    }

    .ingress-tooltip-bold-text {
        font-family: 'font_bold', sans-serif;
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
        color: rgba(83, 95, 119, 0.44);
    }
</style>
