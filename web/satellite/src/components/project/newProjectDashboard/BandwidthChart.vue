// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VChart
        id="bandwidth-chart"
        :key="chartKey"
        :chart-data="chartData"
        :width="width"
        :height="height"
        :tooltip-constructor="tooltip"
    />
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import { ChartData, Tooltip, TooltipParams, TooltipModel, ChartTooltipData } from '@/types/chart';
import { DataStamp } from '@/types/projects';
import { ChartUtils } from '@/utils/chart';

import VChart from '@/components/common/VChart.vue';
import BaseChart from '@/components/common/BaseChart.vue';

// @vue/component
@Component({
    components: { VChart },
})
export default class BandwidthChart extends BaseChart {
    @Prop({ default: () => [] })
    public readonly settledData: DataStamp[];
    @Prop({ default: () => [] })
    public readonly allocatedData: DataStamp[];
    @Prop({ default: new Date() })
    public readonly since: Date;
    @Prop({ default: new Date() })
    public readonly before: Date;

    /**
     * Returns formatted data to render chart.
     */
    public get chartData(): ChartData {
        const mainData: number[] = this.settledData.map(el => el.value);
        const secondaryData: number[] = this.allocatedData.map(el => el.value);
        const xAxisDateLabels: string[] = ChartUtils.daysDisplayedOnChart(this.since, this.before);

        return new ChartData(
            xAxisDateLabels,
            '#FFE0E7',
            '#EE86AD',
            '#FF458B',
            mainData,
            '#FFF6F8',
            '#FFC0CF',
            '#FFC0CF',
            secondaryData,
        );
    }

    /**
     * Used as constructor of custom tooltip.
     */
    public tooltip(tooltipModel: TooltipModel): void {
        if (!tooltipModel.dataPoints) {
            const settledTooltip = Tooltip.createTooltip('settled-bandwidth-tooltip');
            const allocatedTooltip = Tooltip.createTooltip('allocated-bandwidth-tooltip');
            Tooltip.remove(settledTooltip);
            Tooltip.remove(allocatedTooltip);

            return;
        }

        tooltipModel.dataPoints.forEach(p => {
            let tooltipParams: TooltipParams;
            if (p.datasetIndex === 0) {
                tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'settled-bandwidth-tooltip',
                    this.settledTooltipMarkUp(tooltipModel), -20, 78);
            } else {
                tooltipParams = new TooltipParams(tooltipModel, 'bandwidth-chart', 'allocated-bandwidth-tooltip',
                    this.allocatedTooltipMarkUp(tooltipModel), 95, 78);
            }

            Tooltip.custom(tooltipParams);
        });
    }

    /**
     * Returns allocated bandwidth tooltip's html mark up.
     */
    private allocatedTooltipMarkUp(tooltipModel: TooltipModel): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new ChartTooltipData(this.allocatedData[dataIndex]);

        return `<div class='allocated-tooltip'>
                    <p class='settled-tooltip__title'>Allocated</p>
                    <p class='allocated-tooltip__value'>${dataPoint.date}<b class='allocated-tooltip__value__bold'> / ${dataPoint.value}</b></p>
                    <div class='allocated-tooltip__arrow'></div>
                </div>`;
    }

    /**
     * Returns settled bandwidth tooltip's html mark up.
     */
    private settledTooltipMarkUp(tooltipModel: TooltipModel): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new ChartTooltipData(this.settledData[dataIndex]);

        return `<div class='settled-tooltip'>
                    <div class='settled-tooltip__arrow'></div>
                    <p class='settled-tooltip__title'>Settled</p>
                    <p class='settled-tooltip__value'>${dataPoint.date}<b class='settled-tooltip__value__bold'> / ${dataPoint.value}</b></p>
                </div>`;
    }
}
</script>

<style lang="scss">
    .settled-tooltip,
    .allocated-tooltip {
        margin: 8px;
        position: relative;
        border-radius: 14.5px;
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        width: 120px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 17px;
            color: #fff;
            align-self: flex-start;
        }

        &__value {
            font-size: 14px;
            line-height: 26px;
            text-align: center;
            color: #fff;
            white-space: nowrap;
            align-self: flex-start;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }
        }

        &__arrow {
            width: 12px;
            height: 12px;
        }
    }

    .settled-tooltip {
        background-color: #ff458b;
        padding: 4px 10px 8px;

        &__arrow {
            margin: -12px 0 4px;
            border-radius: 0 0 0 8px;
            transform: scale(1, 0.85) translate(0, 20%) rotate(-45deg);
            background-color: #ff458b;
        }
    }

    .allocated-tooltip {
        background-color: #ee86ad;
        padding: 8px 10px 0;

        &__arrow {
            margin-bottom: -4px;
            border-radius: 8px 0 0;
            transform: scale(1, 0.85) translate(0, 20%) rotate(45deg);
            background-color: #ee86ad;
        }
    }
</style>
