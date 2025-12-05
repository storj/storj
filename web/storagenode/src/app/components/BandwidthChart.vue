// Copyright (C) 2019 Storj Labs, Inc.
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
import { ChartUtils } from '@/app/utils/chart';
import { Size } from '@/private/memory/size';
import { BandwidthUsed } from '@/storagenode/sno/sno';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import VChart from '@/app/components/VChart.vue';

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
        this.normalEgress = Size.toBase10String(bandwidth.egress.usage);
        this.normalIngress = Size.toBase10String(bandwidth.ingress.usage);
        this.repairIngress = Size.toBase10String(bandwidth.ingress.repair);
        this.repairEgress = Size.toBase10String(bandwidth.egress.repair);
        this.auditEgress = Size.toBase10String(bandwidth.egress.audit);
        this.date = bandwidth.intervalStart.toUTCString().slice(0, 16);
    }
}

const nodeStore = useNodeStore();

const props = defineProps<{
    width: number;
    height: number;
    isDarkMode: boolean;
}>();

const chartKey = ref<number>(0);

const allBandwidth = computed<BandwidthUsed[]>(() => {
    return ChartUtils.populateEmptyBandwidth(nodeStore.state.bandwidthChartData, BandwidthUsed);
});

const chartBackgroundColor = computed<string>(() => {
    return props.isDarkMode ? '#4F97F7' : '#F2F6FC';
});

const chartDataDimension = computed<string>(() => {
    if (!nodeStore.state.bandwidthChartData.length) {
        return 'Bytes';
    }

    return ChartUtils.getChartDataDimension(allBandwidth.value.map((elem) => {
        return elem.summary();
    }));
});

const chartData = computed<ChartData>(() => {
    let data: number[] = [0];
    if (allBandwidth.value.length) {
        data = ChartUtils.normalizeChartData(allBandwidth.value.map(elem => elem.summary()));
    }

    return {
        labels: ChartUtils.daysDisplayedOnChart(),
        datasets: [
            {
                fill: true,
                backgroundColor: chartBackgroundColor.value,
                borderColor: '#1F49A3',
                borderWidth: 1,
                pointHoverBorderWidth: 3,
                hoverRadius: 8,
                hitRadius: 8,
                pointRadius: 4,
                pointBorderWidth: 1,
                data,
            },
        ],
    };
});

function rebuildChart(): void {
    chartKey.value += 1;
}

function bandwidthTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'bandwidth-tooltip',
        tooltipMarkUp(tooltipModel), 303, 125);

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
            color: var(--regular-text-color);
            margin: 0 0 5px 3px !important;
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
        z-index: 9999;
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
        padding: 10px 0 16px;
        color: var(--regular-text-color);
    }
</style>
