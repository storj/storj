// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <p class="egress-chart__data-dimension">{{ chartDataDimension }}</p>
        <VChart
            :key="chartKey"
            chart-id="egress-chart"
            :chart-data="chartData"
            :width="width"
            :height="height"
            :tooltip-constructor="egressTooltip"
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
 * stores egress data for egress bandwidth chart's tooltip
 */
class EgressTooltip {
    public normalEgress: string;
    public repairEgress: string;
    public auditEgress: string;
    public date: string;

    public constructor(bandwidth: BandwidthRollup) {
        this.normalEgress = Size.toBase10String(bandwidth.egress.usage);
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

    return ChartUtils.getChartDataDimension(allBandwidth.value.map((elem) => elem.egress.audit + elem.egress.repair + elem.egress.usage));
});

const chartData = computed<ChartData>(() => {
    let data: number[] = [0];

    if (allBandwidth.value.length) {
        data = ChartUtils.normalizeChartData(allBandwidth.value.map(elem => elem.egress.audit + elem.egress.repair + elem.egress.usage));
    }

    return {
        labels: ChartUtils.daysDisplayedOnChart(),
        datasets: [
            {
                data,
                fill: true,
                backgroundColor: props.isDarkMode ? '#d2f7e8' : '#edf9f4',
                borderColor: props.isDarkMode ? '#10e089' : '#48a77f',
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

function egressTooltip(tooltipModel: TooltipModel<ChartType>): void {
    const tooltipParams = new TooltipParams(tooltipModel, 'egress-chart', 'egress-tooltip',
        tooltipMarkUp(tooltipModel), 260, 94);

    Tooltip.custom(tooltipParams);
}

function tooltipMarkUp(tooltipModel: TooltipModel<ChartType>): string {
    if (!tooltipModel.dataPoints) {
        return '';
    }

    const dataIndex = tooltipModel.dataPoints[0].dataIndex;
    const dataPoint = new EgressTooltip(allBandwidth.value[dataIndex]);

    return `<div class='egress-tooltip-body'>
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

watch([() => props.isDarkMode, chartData, () => props.width], rebuildChart);
</script>

<style lang="scss">
    .egress-chart {
        z-index: 102;

        &__data-dimension {
            font-size: 13px;
            color: var(--v-header-base);
            margin: 0 0 5px 31px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    #egress-tooltip {
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

    .egress-tooltip-body {
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

    .egress-tooltip-bold-text {
        color: var(--v-success-base);
        font-size: 14px;
    }

    .egress-tooltip-footer {
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
