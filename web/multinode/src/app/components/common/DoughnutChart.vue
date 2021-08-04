// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import * as VueChart from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { DiskStatChartData } from '@/app/types/chart';

@Component({
    extends: VueChart.Doughnut,
})
export default class DoughnutChart extends Vue {
    @Prop({ default: () => new DiskStatChartData() })
    private readonly chartData: DiskStatChartData;

    @Watch('chartData')
    private onDataChange(news: Record<string, unknown>, old: Record<string, unknown>): void {
        (this as any).renderChart(this.chartData, {
            hover: false,
            tooltips: {
                enabled: false,
            },
        });
    }

    public mounted(): void {
        (this as any).renderChart(this.chartData, {
            hover: false,
            tooltips: {
                enabled: false,
            },
        });
    }
}
</script>
