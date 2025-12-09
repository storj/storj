// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <canvas :id="chartId" :width="width" :height="height" />
</template>

<script setup lang="ts">
import {
    CategoryScale,
    Chart as ChartJS,
    LinearScale,
    Tooltip as VTooltip,
    LineController,
    LineElement,
    Filler,
    PointElement,
    TooltipModel,
    ChartType,
    ChartOptions,
    ChartData,
} from 'chart.js';
import { computed, onMounted, onUnmounted, ref } from 'vue';

ChartJS.register(LineElement, PointElement, VTooltip, Filler, LineController, CategoryScale, LinearScale);

const props = defineProps<{
    chartId: string,
    chartData: ChartData;
    width: number,
    height: number,
    tooltipConstructor: (tooltipModel: TooltipModel<ChartType>) => void;
}>();

const chart = ref<ChartJS>();

const chartOptions = computed<ChartOptions>(() => {
    return {
        responsive: true,
        clip: false,
        animation: false,
        layout: {
            padding: {
                top: 15,
                left: 30,
                right: 45,
            },
        },
        elements: {
            line: {
                tension: 0.3,
            },
        },
        scales: {
            y: {
                type: 'linear',
                display: true,
                grid: {
                    display: true,
                    drawTicks: true,
                },
                beginAtZero: true,
            },
            x: {
                type: 'category',
                display: true,
                border: {
                    display: false,
                },
                grid: {
                    display: false,
                },
                ticks: {
                    font: {
                        family: 'sans-serif',
                    },
                    autoSkip: true,
                    maxRotation: 0,
                    minRotation: 0,
                },
            },
        },
        plugins: {
            legend: {
                display: false,
            },
            tooltip: {
                enabled: false,
                external: (context) => {
                    props.tooltipConstructor(context.tooltip);
                },
            },
        },
    };
});

function buildChart(): void {
    chart.value = new ChartJS(
        document.getElementById(props.chartId) as HTMLCanvasElement,
        {
            type: 'line',
            data: props.chartData,
            options: chartOptions.value,
        },
    );
}

onMounted(() => {
    buildChart();
});

onUnmounted(() => {
    chart.value?.destroy();
});
</script>
