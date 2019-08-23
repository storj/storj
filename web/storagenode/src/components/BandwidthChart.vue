// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <Chart
            id="bandwidth-chart"
            :chartData="chartData"
            :width="400"
            :height="200"
            :tooltipConstructor="bandwidthTooltip" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Chart from '@/components/Chart.vue';
    import { ChartUtils } from '@/utils/chart';

    @Component ({
        components: {
            Chart,
        },
    })
    export default class BandwidthChart extends Vue {
        public get chartData(): object {
            let data = [0];

            if (this.$store.state.node.bandwidthChartData.length) {
                data = ChartUtils.normalizeArray(this.$store.state.node.bandwidthChartData.map((elem, index) => {
                    return elem.summary;
                }));
            }

            return {
                labels: ChartUtils.xAxeOptions(new Date()),
                datasets: [{
                    backgroundColor: '#F2F6FC',
                    borderColor: '#1F49A3',
                    borderWidth: 2,
                    data,
                }],
            };
        }

        public bandwidthTooltip(tooltipModel): void {
            // Tooltip Element
            let tooltipEl = document.getElementById('bandwidth-tooltip');
            // Create element on first render
            if (!tooltipEl) {
                tooltipEl = document.createElement('div');
                tooltipEl.id = 'bandwidth-tooltip';
                document.body.appendChild(tooltipEl);
            }

            // Hide if no tooltip
            if (tooltipModel.opacity === 0) {
                document.body.removeChild(tooltipEl);

                return;
            }

            // Set Text
            if (tooltipModel.body) {
                const dataIndex = tooltipModel.dataPoints[0].index;
                const dataPoint = this.$store.state.node.bandwidthChartData[dataIndex].getLabels();
                tooltipEl.innerHTML = `<div class='tooltip-header'>
                                           <p>EGRESS</p>
                                           <p class='tooltip-header__ingress'>INGRESS</p>
                                       </div>
                                       <div class='tooltip-body'>
                                           <div class='tooltip-body__info'>
                                               <p>NORMAL</p>
                                               <p class='tooltip-body__info__egress-value'><b>${dataPoint.normalEgress}</b></p>
                                               <p class='tooltip-body__info__ingress-value'><b>${dataPoint.normalIngress}</b></p>
                                           </div>
                                           <div class='tooltip-body__info'>
                                               <p>REPAIR</p>
                                               <p class='tooltip-body__info__egress-value'><b>${dataPoint.repairEgress}</b></p>
                                               <p class='tooltip-body__info__ingress-value'><b>${dataPoint.repairIngress}</b></p>
                                           </div>
                                           <div class='tooltip-body__info'>
                                               <p>AUDIT</p>
                                               <p class='tooltip-body__info__egress-value'><b>${dataPoint.auditEgress}</b></p>
                                           </div>
                                       </div>
                                       <div class='tooltip-footer'>
                                           <p>${dataPoint.date}</p>
                                       </div>`;
            }

            // `this` will be the overall tooltip
            let bandwidthChart = document.getElementById('bandwidth-chart');
            if (bandwidthChart) {
                let position = bandwidthChart.getBoundingClientRect();
                tooltipEl.style.opacity = '1';
                tooltipEl.style.position = 'absolute';
                tooltipEl.style.left = position.left + tooltipModel.caretX + 'px';
                tooltipEl.style.top = position.top + window.pageYOffset + tooltipModel.caretY + 'px';
            }

            return;
        }
    }
</script>

<style lang="scss">
    #bandwidth-tooltip {
        background-color: #FFFFFF;
        width: auto;
        font-family: 'font_regular';
        font-size: 12px;
        border-radius: 8px;
        box-shadow: 0 2px 10px #D2D6DE;
        color: #535F77;
        padding: 6px;
        pointer-events: none;
    }

    .tooltip-header {
        display: flex;
        padding: 0 35px 0 83px;
        line-height: 57px;

        &__ingress {
            margin-left: 30px;
        }
    }

    .tooltip-body {

        &__info {
            display: flex;
            background-color: #EBECF0;
            border-radius: 12px;
            padding: 14px 17px 14px 14px;
            align-items: center;
            margin-bottom: 14px;
            position: relative;

            b {
                font-size: 14px;
            }

            &__egress-value {
                position: absolute;
                left: 83px;
            }

            &__ingress-value {
                position: absolute;
                left: 158px;
            }
        }
    }

    .tooltip-footer {
        font-size: 12px;
        width: auto;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 10px 0 16px 0;
    }
</style>
