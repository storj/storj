// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bandwidth">
        <h1 class="bandwidth__title">Bandwidth & Disk</h1>
        <div class="bandwidth__dropdowns">
            <node-selection-dropdown />
            <satellite-selection-dropdown />
        </div>
        <div class="chart-container bandwidth-chart">
            <div class="chart-container__title-area">
                <p class="chart-container__title-area__title">Bandwidth Used This Month</p>
                <div class="chart-container__title-area__buttons-area">
                    <button
                        name="Show Bandwidth Chart"
                        class="chart-container__title-area__chart-choice-item"
                        type="button"
                        :class="{ 'active': (!isEgressChartShown && !isIngressChartShown) }"
                        @click.stop="openBandwidthChart"
                    >
                        Bandwidth
                    </button>
                    <button
                        name="Show Egress Chart"
                        class="chart-container__title-area__chart-choice-item"
                        type="button"
                        :class="{ 'active': isEgressChartShown }"
                        @click.stop="openEgressChart"
                    >
                        Egress
                    </button>
                    <button
                        name="Show Ingress Chart"
                        class="chart-container__title-area__chart-choice-item"
                        type="button"
                        :class="{ 'active': isIngressChartShown }"
                        @click.stop="openIngressChart"
                    >
                        Ingress
                    </button>
                </div>
            </div>
            <p v-if="isEgressChartShown" class="chart-container__amount"><b>{{ bandwidth.egressSummary | bytesToBase10String }}</b></p>
            <p v-else-if="isIngressChartShown" class="chart-container__amount"><b>{{ bandwidth.ingressSummary | bytesToBase10String }}</b></p>
            <p v-else class="chart-container__amount"><b>{{ bandwidth.bandwidthSummary | bytesToBase10String }}</b></p>
            <div ref="chart" class="chart-container__chart" onresize="recalculateChartDimensions()">
                <egress-chart v-if="isEgressChartShown" :height="chartHeight" :width="chartWidth" />
                <ingress-chart v-else-if="isIngressChartShown" :height="chartHeight" :width="chartWidth" />
                <bandwidth-chart v-else :height="chartHeight" :width="chartWidth" />
            </div>
        </div>
        <section class="bandwidth__chart-area">
            <section class="chart-container">
                <div class="chart-container__title-area disk-space-title">
                    <p class="chart-container__title-area__title">Average Disk Space Used This Month</p>
                </div>
                <p class="chart-container__amount disk-space-amount"><b>{{ diskSpaceUsageSummary | bytesToBase10String }}</b></p>
                <div ref="diskSpaceChart" class="chart-container__chart" onresize="recalculateChartDimensions()">
                    <disk-space-chart :height="diskSpaceChartHeight" :width="diskSpaceChartWidth" />
                </div>
            </section>
            <section class="disk-stat-chart">
                <disk-stat-chart />
            </section>
        </section>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { UnauthorizedError } from '@/api';
import { BandwidthTraffic } from '@/bandwidth';

import BandwidthChart from '@/app/components/bandwidth/BandwidthChart.vue';
import EgressChart from '@/app/components/bandwidth/EgressChart.vue';
import IngressChart from '@/app/components/bandwidth/IngressChart.vue';
import NodeSelectionDropdown from '@/app/components/common/NodeSelectionDropdown.vue';
import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import DiskSpaceChart from '@/app/components/storage/DiskSpaceChart.vue';
import DiskStatChart from '@/app/components/storage/DiskStatChart.vue';

// @vue/component
@Component({
    components: {
        DiskStatChart,
        DiskSpaceChart,
        EgressChart,
        IngressChart,
        BandwidthChart,
        NodeSelectionDropdown,
        SatelliteSelectionDropdown,
    },
})
export default class BandwidthPage extends Vue {
    public chartWidth = 0;
    public chartHeight = 0;
    public diskSpaceChartWidth = 0;
    public diskSpaceChartHeight = 0;
    public isEgressChartShown = false;
    public isIngressChartShown = false;
    public $refs: {
        chart: HTMLElement;
        diskSpaceChart: HTMLElement;
    };

    public get bandwidth(): BandwidthTraffic {
        return this.$store.state.bandwidth.traffic;
    }

    public get diskSpaceUsageSummary(): number {
        return this.$store.state.storage.usage.diskSpaceSummaryBytes;
    }

    /**
     * Used container size recalculation for charts resizing.
     */
    public recalculateChartDimensions(): void {
        this.chartWidth = this.$refs.chart.clientWidth;
        this.chartHeight = this.$refs.chart.clientHeight;
        this.diskSpaceChartWidth = this.$refs.diskSpaceChart.clientWidth;
        this.diskSpaceChartHeight = this.$refs.diskSpaceChart.clientHeight;
    }

    /**
     * Lifecycle hook after initial render.
     * Adds event on window resizing to recalculate size of charts.
     */
    public async mounted(): Promise<void> {
        window.addEventListener('resize', this.recalculateChartDimensions);

        try {
            await this.$store.dispatch('nodes/fetchOnline');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }

        await this.fetchTraffic();

        // Subscribes on period or satellite change
        this.$store.subscribe(async(mutation) => {
            const watchedMutations = ['nodes/setSelectedNode', 'nodes/setSelectedSatellite'];

            if (watchedMutations.includes(mutation.type)) {
                await this.fetchTraffic();
            }
        });

        this.recalculateChartDimensions();
    }

    /**
     * Lifecycle hook before component destruction.
     * Removes event on window resizing.
     */
    public beforeDestroy(): void {
        window.removeEventListener('resize', this.recalculateChartDimensions);
    }

    /**
     * Changes bandwidth chart source to summary of ingress and egress.
     */
    public openBandwidthChart(): void {
        this.isEgressChartShown = false;
        this.isIngressChartShown = false;
    }

    /**
     * Changes bandwidth chart source to ingress.
     */
    public openIngressChart(): void {
        this.isEgressChartShown = false;
        this.isIngressChartShown = true;
    }

    /**
     * Changes bandwidth chart source to egress.
     */
    public openEgressChart(): void {
        this.isEgressChartShown = true;
        this.isIngressChartShown = false;
    }

    /**
     * Fetches bandwidth and disk space information.
     */
    private async fetchTraffic(): Promise<void> {
        try {
            await this.$store.dispatch('bandwidth/fetch');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }

        try {
            await this.$store.dispatch('storage/usage');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }

        try {
            await this.$store.dispatch('storage/diskSpace');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }
    }
}
</script>

<style lang="scss" scoped>
    .bandwidth {
        box-sizing: border-box;
        padding: 60px;
        height: 100%;
        overflow-y: auto;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--c-title);
            margin-bottom: 44px;
        }

        &__dropdowns {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 70%;

            & > *:first-of-type {
                margin-right: 20px;
            }

            .dropdown {
                max-width: unset;
            }
        }

        & .chart-container {
            box-sizing: border-box;
            width: 65%;
            height: 401px;
            background-color: white;
            border: 1px solid var(--c-gray--light);
            border-radius: 11px;
            padding: 32px 30px;
            margin: 20px 0 13px;
            position: relative;

            &__title-area {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__buttons-area {
                    display: flex;
                    flex-direction: row;
                    align-items: flex-end;
                }

                &__title {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    color: var(--c-gray);
                    user-select: none;
                }

                &__chart-choice-item {
                    padding: 6px 8px;
                    background-color: #e7e9eb;
                    border-radius: 6px;
                    font-size: 12px;
                    color: #586474;
                    max-height: 25px;
                    cursor: pointer;
                    user-select: none;
                    margin-left: 9px;
                    border: none;

                    &.active {
                        background-color: #d5d9dc;
                        color: #131d3a;
                    }
                }
            }

            &__amount {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 57px;
                color: var(--c-title);
            }

            &__chart {
                position: absolute;
                left: 0;
                width: calc(100% - 10px);
                height: 240px;
            }
        }

        &__chart-area {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            width: 100%;
        }
    }

    .disk-space-amount {
        margin-top: 5px;
    }

    .bandwidth-chart {
        width: 100% !important;
    }

    .disk-stat-chart {
        margin: 20px 0 13px;
        width: auto;
    }
</style>
