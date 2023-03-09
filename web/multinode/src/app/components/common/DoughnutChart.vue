// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import * as VueChart from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { DiskStatChartData, RenderChart } from '@/app/types/chart';

// @vue/component
@Component({
    extends: VueChart.Doughnut,
})
export default class DoughnutChart extends Vue {
    @Prop({ default: () => new DiskStatChartData() })
    private readonly chartData: DiskStatChartData;

    @Watch('chartData')
    private onDataChange(_news: Record<string, unknown>, _old: Record<string, unknown>): void {
        (this as unknown as RenderChart).renderChart(this.chartData, {
            hover: false,
            tooltips: {
                enabled: false,
            },
        });
    }

    public mounted(): void {
        (this as unknown as RenderChart).renderChart(this.chartData, {
            hover: false,
            tooltips: {
                enabled: false,
            },
        });
    }
}
</script>
