// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import VueChartJs from 'vue-chartjs';

    @Component ({
        extends: VueChartJs.Line,
        props: {
            currency: {
                type: String,
                default: '$'
            },
            max: Number,
            min: Number,
            tooltipConstructor: Function,
            chartData: Object,
        },
        computed: {
            chartOptions: function() {
                return {
                    responsive: false,
                    maintainAspectRatios: false,

                    legend: {
                        display: false,
                    },

                    elements: {
                        point: {
                            radius: 3,
                            hitRadius: 10,
                            hoverRadius: 10,
                            hoverBackgroundColor: '#4D72B7',
                        }
                    },

                    scales: {
                        yAxes: [{
                            display: true,
                            ticks: {
                                fontFamily: 'font_regular',
                                max: parseInt(this.$props.max),
                                min: parseInt(this.$props.min)
                            },
                            gridLines: {
                                display:false
                            },
                        }],
                        xAxes: [{
                            display: true,
                            gridLines: {
                                display:false
                            },
                        }],
                    },

                    tooltips: {
                        enabled: false,

                        custom: ((tooltipModel) => {
                            this.$props.tooltipConstructor(tooltipModel);
                        }),

                        labels: {
                            enabled: true,
                        }
                    }
                }
            }
        },

        mixins: [VueChartJs.mixins.reactiveProp],
        mounted() {
            (this as any).renderChart(this.$props.chartData, (this as any).chartOptions);
        }
    })

    export default class Chart extends Vue {}
</script>

<style lang="scss">

</style>
