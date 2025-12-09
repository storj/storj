// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <p class="bandwidth-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            :key="chartKey"
            chart-id="bandwidth-chart"
            :chart-data="chartData"
            :width="width"
            :height="height"
            :tooltip-constructor="bandwidthTooltip"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ChartData, ChartType, TooltipModel } from 'chart.js';

import { Tooltip, TooltipParams } from '@/app/types/chart';
import { Chart as ChartUtils } from '@/app/utils/chart';
import { BandwidthRollup } from '@/bandwidth';
import { Size } from '@/private/memory/size';
import { useBandwidthStore } from '@/app/store/bandwidthStore';

import VChart from '@/app/components/common/VChart.vue';

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

const bandwidthStore = useBandwidthStore();

const props = defineProps<{
    width: number;
    height: number;
    isDarkMode: boolean;
}>();

const chartKey = ref<number>(0);

const allBandwidth = computed<BandwidthRollup[]>(() => ChartUtils.populateEmptyBandwidth(bandwidthStore.state.traffic.bandwidthDaily));

const chartDataDimension = computed<string>(() => {
    if (!bandwidthStore.state.traffic.bandwidthDaily.length) {
        return 'Bytes';
    }

    return ChartUtils.getChartDataDimension(allBandwidth.value.map((elem) => elem.egress.usage + elem.egress.repair + elem.egress.audit
            + elem.ingress.repair + elem.ingress.usage));
});

const chartData = computed<ChartData>(() => {
    let data: number[] = [0];
    if (allBandwidth.value.length) {
        data = ChartUtils.normalizeChartData(allBandwidth.value.map(elem => elem.egress.usage + elem.egress.repair + elem.egress.audit
                + elem.ingress.repair + elem.ingress.usage));
    }

    return {
        labels: ChartUtils.daysDisplayedOnChart(),
        datasets: [
            {
                data,
                fill: true,
                backgroundColor: props.isDarkMode ? '#d4effa' : '#F2F6FC',
                borderColor: props.isDarkMode ? '#0052FF' : '#1F49A3',
                borderWidth: 1,
                pointHoverBorderWidth: 3,
                hoverRadius: 8,
                hitRadius: 8,
                pointRadius: 4,
                pointBorderWidth: 1,
            },
        ],
    };
});

function rebuildChart(): void {
    chartKey.value += 1;
}

function bandwidthTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'bandwidth-tooltip',
        tooltipMarkUp(tooltipModel), 300, 125);

    Tooltip.custom(tooltipParams);
}

function tooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new BandwidthTooltip(allBandwidth.value[dataIndex]);

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

watch([() => props.isDarkMode, chartData, () => props.width], rebuildChart);
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
