// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import * as VueChart from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { ChartData } from '@/app/types/chartData';

@Component({
    extends: VueChart.Line
})
export default class VChart extends Vue {
    @Prop({default: '$'})
    private readonly currency: string;
    @Prop({default: () => { console.error('Tooltip constructor is undefined'); }, })
    private tooltipConstructor: (tooltipModel) => void;
    @Prop({default: {}})
    private readonly chartData: ChartData;

    @Watch('chartData')
    private onDataChange(news: object, old: object) {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as any).renderChart(this.chartData, this.chartOptions);
    }

    public mounted(): void {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as any).renderChart(this.chartData, this.chartOptions);
    }

    public get chartOptions(): object {
        return {
            responsive: false,
            maintainAspectRatios: false,

            legend: {
                display: false,
            },

            elements: {
                point: {
                    radius: 0,
                    hitRadius: 5,
                    hoverRadius: 5,
                    hoverBackgroundColor: '#4D72B7',
                }
            },

            scales: {
                yAxes: [{
                    display: false,
                    ticks: {
                        beginAtZero: true
                    }
                }],
                xAxes: [{
                    display: true,
                    ticks: {
                        fontFamily: 'font_regular',
                        autoSkip: true,
                        maxRotation: 0,
                        minRotation: 0,
                    },
                    gridLines: {
                        display: false
                    },
                }],
            },

            tooltips: {
                enabled: false,

                custom: ((tooltipModel) => {
                    this.tooltipConstructor(tooltipModel);
                }),

                labels: {
                    enabled: true,
                }
            }
        };
    }
}
</script>

<style lang="scss"></style>
