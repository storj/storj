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
                    display: true,
                    ticks: {
                        beginAtZero: true,
                    },
                    gridLines: {
                        borderDash: [2, 5],
                        drawBorder: false
                    }
                }],
                xAxes: [{
                    display: true,
                    ticks: {
                        fontFamily: 'font_regular',
                        autoSkip: false,
                        maxRotation: 0,
                        minRotation: 0,
                        callback: function(value, index, values): string | undefined {
                            const valuesLength = values.length;
                            const firstValue = values[0];
                            const lastValue = values[valuesLength - 1];
                            const middleIndex = valuesLength / 2;
                            const isAfterEighthDayOfTheMonth = valuesLength > 8 && valuesLength <= 31;

                            if (valuesLength <= 8) {
                                return value
                            }

                            if (value === firstValue || value === lastValue ||
                                (isAfterEighthDayOfTheMonth && valuesLength % 2 === 0
                                && index === (middleIndex - 1))) {
                                return value;
                            }

                            if (value === firstValue || value === lastValue ||
                                (isAfterEighthDayOfTheMonth && valuesLength % 2 !== 0
                                && index === (Math.floor(middleIndex)))) {
                                return value;
                            }
                        }
                    },
                    gridLines: {
                        display: false,
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
