// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import * as VueChart from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { DiskStatChartData } from '@/app/types/chartData';

@Component({
    extends: VueChart.Doughnut,
})
export default class DoughnutChart extends Vue {
    @Prop({default: () => new DiskStatChartData()})
    private readonly chartData: DiskStatChartData;

    @Watch('chartData')
    private onDataChange(news: object, old: object): void {
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
