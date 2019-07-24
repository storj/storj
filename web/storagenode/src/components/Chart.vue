// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import * as VChart from 'vue-chartjs';

    @Component({
        extends: VChart.Line
    })
    export default class Chart extends Vue {
        @Prop({default: '$'})
        private readonly currency: string;
        @Prop({default: ''})
        private readonly max: string;
        @Prop({default: ''})
        private readonly min: string;
        @Prop({default: () => { console.error('Tooltip constructor is undefined'); }, })
        private tooltipConstructor: (tooltipModel) => void;
        @Prop({default: {}})
        private readonly chartData: object;

        public mounted(): void {
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
                        radius: 3,
                        hitRadius: 5,
                        hoverRadius: 5,
                        hoverBackgroundColor: '#4D72B7',
                    }
                },

                scales: {
                    yAxes: [{
                        display: true,
                        ticks: {
                            fontFamily: 'font_regular',
                            max: parseInt(this.max),
                            min: parseInt(this.min),
                            autoSkip: false,
                        },
                        gridLines: {
                            display:false
                        },
                    }],
                    xAxes: [{
                        display: true,
                        ticks: {
                            autoSkip: false,
                        },
                        gridLines: {
                            display:false
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
