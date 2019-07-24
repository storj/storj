// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="chart">
        <Chart
            id="disk-space-chart"
            :chartData="diskSpaceUsed"
            :width="400"
            :height="150"
            min="1"
            max="7.2"
            :tooltipConstructor="diskSpaceTooltip" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Chart from '@/components/Chart.vue';

    @Component ({
        components: {
            Chart,
        },
    })
    export default class DiskSpaceChart extends Vue {
        public diskSpaceUsed: object = {
            labels: [
                '1',
                '',
                '',
                '',
                '',
                '',
                '15',
                '',
                '',
                '',
                '',
                '30'
                ],
            datasets: [{
                backgroundColor: '#F2F6FC',
                borderColor: '#1F49A3',
                borderWidth: 2,
                data: [4, 3, 5, 5, 4, 3, 3, 6, 5, 4, 5, 2],
            }],
        };

        public diskSpaceTooltip(tooltipModel): void {
            // Tooltip Element
            let tooltipEl = document.getElementById('disk-space-tooltip');
            // Create element on first render
            if (!tooltipEl) {
                tooltipEl = document.createElement('div');
                tooltipEl.id = 'disk-space-tooltip';
                document.body.appendChild(tooltipEl);
            }

            // Hide if no tooltip
            if (tooltipModel.opacity === 0) {
                document.body.removeChild(tooltipEl);

                return;
            }

            // Set Text
            if (tooltipModel.body) {
                tooltipEl.innerHTML = `<div class='tooltip-body'>
                                           <p class='tooltip-body__data'><b>30GB</b></p>
                                           <p class='tooltip-body__footer'>May 25, 2019</p>
                                       </div>`;
            }

            let diskSpaceChart = document.getElementById('disk-space-chart');

            if (diskSpaceChart) {
                let position = diskSpaceChart.getBoundingClientRect();
                tooltipEl.style.opacity = '1';
                tooltipEl.style.position = 'absolute';
                tooltipEl.style.right = position.left + window.pageXOffset - tooltipModel.caretX - 20 + 'px';
                tooltipEl.style.top = position.top + window.pageYOffset + tooltipModel.caretY + 'px';
            }

            return;
        }
    }
</script>

<style lang="scss">
    #disk-space-tooltip {
        background-color: #FFFFFF;
        width: auto;
        font-size: 12px;
        font-family: 'font_regular';
        border-radius: 8px;
        box-shadow: 0 2px 10px #D2D6DE;
        color: #535F77;
        padding: 6px;
        pointer-events: none;
    }

    .tooltip-body {
        &__data {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 11px 44px 11px 44px;
            font-size: 14px;
        }

        &__footer {
            font-size: 12px;
            width: auto;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 10px 0 16px 0;
        }
    }
</style>
