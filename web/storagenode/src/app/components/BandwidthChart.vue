// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <VChart
            id="bandwidth-chart"
            :chart-data="chartData"
            :width="400"
            :height="240"
            :tooltip-constructor="bandwidthTooltip"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VChart from '@/app/components/VChart.vue';
import { ChartData } from '@/app/types/chartData';
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
        this.date = bandwidth.intervalStart.toUTCString();
    }
}

@Component ({
    components: {
        VChart,
    },
})
export default class BandwidthChart extends Vue {
    private get allBandwidth(): BandwidthUsed[] {
        return ChartUtils.populateEmptyBandwidth(this.$store.state.node.bandwidthChartData);
    }

    public get chartData(): ChartData {
        let data: number[] = [0];
        const daysCount = ChartUtils.daysDisplayedOnChart(new Date());
        const chartBackgroundColor = '#F2F6FC';
        const chartBorderColor = '#1F49A3';
        const chartBorderWidth = 2;

        if (this.allBandwidth.length) {
            data = ChartUtils.normalizeChartData(this.allBandwidth.map((elem) => {
                return elem.summary();
            }));
        }

        return new ChartData(daysCount, chartBackgroundColor, chartBorderColor, chartBorderWidth, data);
    }

    public bandwidthTooltip(tooltipModel): void {
        // Tooltip Element
        let tooltipEl = document.getElementById('bandwidth-tooltip');
        // Create element on first render
        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.id = 'bandwidth-tooltip';
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
            const dataPoint = new BandwidthTooltip(this.allBandwidth[dataIndex]);

            tooltipEl.innerHTML = `<div class='tooltip-header'>
                                       <p>EGRESS</p>
                                       <p class='tooltip-header__ingress'>INGRESS</p>
                                   </div>
                                   <div class='tooltip-body'>
                                       <div class='tooltip-body__info'>
                                           <p>NORMAL</p>
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

        // `this` will be the overall tooltip
        const bandwidthChart = document.getElementById('bandwidth-chart');
        if (bandwidthChart) {
            const position = bandwidthChart.getBoundingClientRect();
            tooltipEl.style.opacity = '1';
            tooltipEl.style.position = 'absolute';
            tooltipEl.style.left = position.left + tooltipModel.caretX + 'px';
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

    #bandwidth-tooltip {
        background-color: #FFFFFF;
        width: auto;
        font-size: 12px;
        border-radius: 8px;
        box-shadow: 0 2px 10px #D2D6DE;
        color: #535F77;
        padding: 6px;
        pointer-events: none;
    }

    .tooltip-header {
        display: flex;
        padding: 0 35px 0 83px;
        line-height: 57px;

        &__ingress {
            margin-left: 30px;
        }
    }

    .tooltip-body {

        &__info {
            display: flex;
            background-color: #EBECF0;
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
    }
</style>
