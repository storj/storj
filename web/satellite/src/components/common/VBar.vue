// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bar-container">
        <div class="bar-container__fill" :style="barFillStyle" />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

/**
 * BarFillStyle class holds info for BarFillStyle entity.
 */
class BarFillStyle {
    'background-color': string;
    width: string;

    public constructor(backgroundColor: string, width: string) {
        this['background-color'] = backgroundColor;
        this.width = width;
    }
}

const props = withDefaults(defineProps<{
    current?: number;
    max?: number;
    color?: string;
}>(), {
    current: 0,
    max: 0,
    color: '#0068DC',
});

const barFillStyle = computed((): BarFillStyle => {
    if (props.current > props.max) {
        return new BarFillStyle(props.color, '100%');
    }

    const width = (props.current / props.max) * 100 + '%';

    return new BarFillStyle(props.color, width);
});
</script>

<style scoped lang="scss">
    .bar-container {
        width: 100%;
        height: 13px;
        margin-top: 10px;
        border-radius: 4px;
        background-color: #f4f6f9;
        position: relative;

        &__fill {
            max-width: 100%;
            height: 100%;
            position: absolute;
            left: 0;
            top: 0;
            border-radius: 20px;
        }
    }
</style>

