// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VChart
        id="storage-chart"
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
export default class StorageChart extends BaseChart {
    @Prop({ default: () => [] })
    public readonly data: DataStamp[];
    @Prop({ default: new Date() })
    public readonly since: Date;
    @Prop({ default: new Date() })
    public readonly before: Date;

    /**
     * Returns formatted data to render chart.
     */
    public get chartData(): ChartData {
        const data: number[] = this.data.map(el => el.value);
        const xAxisDateLabels: string[] = ChartUtils.daysDisplayedOnChart(this.since, this.before);

        return new ChartData(
            xAxisDateLabels,
            '#E6EDF7',
            '#D7E8FF',
            '#003DC1',
            data,
        );
    }

    /**
     * Used as constructor of custom tooltip.
     */
    public tooltip(tooltipModel: TooltipModel): void {
        const tooltipParams = new TooltipParams(tooltipModel, 'storage-chart', 'storage-tooltip',
            this.tooltipMarkUp(tooltipModel), 76, 81);

        Tooltip.custom(tooltipParams);
    }

    /**
     * Returns tooltip's html mark up.
     */
    private tooltipMarkUp(tooltipModel: TooltipModel): string {
        if (!tooltipModel.dataPoints) {
            return '';
        }

        const dataIndex = tooltipModel.dataPoints[0].index;
        const dataPoint = new ChartTooltipData(this.data[dataIndex]);

        return `<div class='tooltip'>
                    <p class='tooltip__value'>${dataPoint.date}<b class='tooltip__value__bold'> / ${dataPoint.value}</b></p>
                    <div class='tooltip__arrow' />
                </div>`;
    }
}
</script>

<style lang="scss">
    .tooltip {
        margin: 8px;
        position: relative;
        box-shadow: 0 5px 14px rgb(9 87 203 / 26%);
        border-radius: 100px;
        padding-top: 8px;
        width: 145px;
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        background-color: #003dc1;

        &__value {
            font-size: 14px;
            line-height: 26px;
            text-align: center;
            color: #fff;
            white-space: nowrap;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }
        }

        &__arrow {
            width: 12px;
            height: 12px;
            border-radius: 8px 0 0;
            transform: scale(1, 0.85) translate(0, 20%) rotate(45deg);
            margin-bottom: -4px;
            background-color: #003dc1;
        }
    }
</style>
