// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <Chart id="bandwidth-chart" :chartData="bandwidthUsed" :width="400" :height="150" min="1" max="7.2"
               tooltipHTML="<div class='tooltip-header'>
                                  <p>EGRESS</p>
                                  <p class='tooltip-header__ingress'>INGRESS</p>
                              </div>
                              <div class='tooltip-body'>
                                  <div class='tooltip-body__info'>
                                      <p>NORMAL</p>
                                      <p class='tooltip-body__info__egress-value'><b>40GB</b></p>
                                      <p class='tooltip-body__info__ingress-value'><b>40GB</b></p>
                                  </div>
                                  <div class='tooltip-body__info'>
                                      <p>REPAIR</p>
                                      <p class='tooltip-body__info__egress-value'><b>20GB</b></p>
                                      <p class='tooltip-body__info__ingress-value'><b>20GB</b></p>
                                  </div>
                                  <div class='tooltip-body__info'>
                                      <p>AUDIT</p>
                                      <p class='tooltip-body__info__egress-value'><b>30GB</b></p>
                                  </div>
                              </div>
                              <div class='tooltip-footer'>
                                  <p>May 25, 2019</p>
                              </div>"
               :tooltipConstructor = "tooltip" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Chart from '@/components/Chart.vue';

    @Component ({
        components: {
            Chart,
        },
        data: () => ({
            bandwidthUsed: {
                labels: [],
                datasets: [{
                    backgroundColor: '#F2F6FC',
                    borderColor: '#1F49A3',
                    borderWidth: 2,
                    data: Array(27).fill(5),
                }],
            },
        }),
        mounted: function () {
            this.$data.bandwidthUsed.labels = (this as any).xAxeOption()
        },
        methods: {
            tooltip: function (tooltipModel, html): void {
                // Tooltip Element
                var tooltipEl = document.getElementById('bandwidth-tooltip');
                // Create element on first render
                if (!tooltipEl) {
                    tooltipEl = document.createElement('div');
                    tooltipEl.id = 'bandwidth-tooltip';
                    document.body.appendChild(tooltipEl);
                }

                // Hide if no tooltip
                if (tooltipModel.opacity === 0) {
                    tooltipEl.style.opacity = '0';
                    return;
                }

                // Set Text
                if (tooltipModel.body) {
                    tooltipEl.innerHTML = html;
                }

                // `this` will be the overall tooltip
                var bandwidthChart = document.getElementById('bandwidth-chart');
                if(bandwidthChart) {
                    var position = bandwidthChart.getBoundingClientRect();
                    tooltipEl.style.opacity = '1';
                    tooltipEl.style.position = 'absolute';
                    tooltipEl.style.left = position.left + window.pageXOffset + tooltipModel.caretX + 'px';
                    tooltipEl.style.top = position.top + window.pageYOffset + tooltipModel.caretY + 'px';
                }
            },

            xAxeOption: function () {
                let dateNow = new Date().getDate();
                let daysDisplayed = (dateNow === 1) ? Array(dateNow + 1).fill('') : Array(dateNow).fill('');

                daysDisplayed[0] = 1;
                daysDisplayed[dateNow - 1] = dateNow;
                if (dateNow > 2) {
                    daysDisplayed[Math.round(dateNow/2)] = Math.floor(dateNow/2);
                }

                return daysDisplayed;
            }
        }
    })

    export default class BandwidthChart extends Vue {

    }
</script>

<style lang="scss">
    .chart {
        position: relative;
    }

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
