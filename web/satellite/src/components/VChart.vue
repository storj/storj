// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <canvas :id="chartId" :width="width" :height="height" />
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
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
    Plugin,
} from 'chart.js';

import { TooltipId } from '@/types/chart';

ChartJS.register(LineElement, PointElement, VTooltip, Filler, LineController, CategoryScale, LinearScale);

const props = defineProps<{
    chartId: string,
    chartData: ChartData,
    dataLabel: string,
    tooltipConstructor: (tooltipModel: TooltipModel<ChartType>) => void,
    width: number,
    height: number,
}>();

const chart = ref<ChartJS>();

/**
 * Returns a plugin which draws a dashed line under active datapoint.
 */
const afterDatasetsDrawPlugin = computed((): Plugin => {
    return {
        id: 'afterDatasetsDraw',
        afterDatasetsDraw: (chart) => {
            if (chart.tooltip) {
                const activePoint = chart.tooltip.getActiveElements();
                if (activePoint[0]) {
                    const ctx = chart.ctx;
                    const yAxis = chart.scales['y'];
                    const tooltipPosition = activePoint[0].element.tooltipPosition(true);

                    ctx.save();
                    ctx.beginPath();
                    ctx.setLineDash([8, 5]);
                    ctx.moveTo(tooltipPosition.x, tooltipPosition.y + 12);
                    ctx.lineTo(tooltipPosition.x, yAxis.bottom);
                    ctx.lineWidth = 1;
                    ctx.strokeStyle = '#C8D3DE';
                    ctx.stroke();
                    ctx.restore();
                }
            }
        },
    };
});

/**
 * Returns chart options.
 */
const chartOptions = computed((): ChartOptions => {
    return {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        clip: false,
        layout: {
            padding: {
                top: 25,
                left: 10,
                right: 40,
                bottom: 15,
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
                border: {
                    display: false,
                },
                grid: {
                    display: false,
                },
                suggestedMin: 0,
                suggestedMax: 150,
                ticks: {
                    font: {
                        family: 'sans-serif',
                    },
                    maxTicksLimit: 5,
                    callback: function(value, _, ticks) {
                        const numDigits = ticks[ticks.length - 2].value.toString().length;

                        const power = Math.floor((numDigits - 1) / 3);
                        const val =  (value as number) / Math.pow(1000, power);
                        return `${val}${props.dataLabel}`;
                    },
                },
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

onMounted(() => {
    chart.value = new ChartJS(
        document.getElementById(props.chartId) as HTMLCanvasElement,
        {
            type: 'line',
            data: props.chartData,
            options: chartOptions.value,
            plugins: [afterDatasetsDrawPlugin.value],
        },
    );
});

onUnmounted(() => {
    chart.value?.destroy();

    // custom tooltip element doesn't get cleaned up if the user navigates to a new page using the keyboard.
    // There is probably a better way to do this
    const storageTooltip = document.getElementById(TooltipId.Storage);
    if (storageTooltip) {
        document.body.removeChild(storageTooltip);
    }

    const egressTooltip = document.getElementById(TooltipId.Bandwidth);
    if (egressTooltip) {
        document.body.removeChild(egressTooltip);
    }
});

watch(() => props.chartData, () => {
    chart.value?.destroy();
    chart.value = new ChartJS(
        document.getElementById(props.chartId) as HTMLCanvasElement,
        {
            type: 'line',
            data: props.chartData,
            options: chartOptions.value,
            plugins: [afterDatasetsDrawPlugin.value],
        },
    );
});
</script>
