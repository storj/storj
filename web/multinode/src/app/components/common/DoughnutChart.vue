// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <canvas :id="chartId" class="chart" />
</template>

<script setup lang="ts">
import {
    registerables,
    Chart as ChartJS,
    ChartData,
} from 'chart.js';
import { onMounted, onUnmounted, ref, watch } from 'vue';

ChartJS.register(...registerables);

const props = defineProps<{
    chartId: string;
    chartData: ChartData;
}>();

const chart = ref<ChartJS>();

function buildChart(): void {
    chart.value = new ChartJS(
        document.getElementById(props.chartId) as HTMLCanvasElement,
        {
            type: 'doughnut',
            data: props.chartData,
            options: {
                plugins: {
                    tooltip: {
                        enabled: false,
                    },
                },
                responsive: false,
                clip: false,
                animation: false,
                maintainAspectRatio: false,
                hover: {
                    mode: null,
                },
            },
        },
    );
}

onMounted(() => {
    buildChart();
});

onUnmounted(() => {
    chart.value?.destroy();
});

watch(() => props.chartData, () => {
    chart.value?.destroy();
    buildChart();
});
</script>

<style scoped lang="scss">
.chart {
    width: 220px !important;
    height: 220px !important;
}

@media screen and (width <= 1000px) {

    .chart {
        width: 150px !important;
        height: 150px !important;
    }
}
</style>
