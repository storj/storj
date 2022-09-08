// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bar-container">
        <div class="bar-container__fill" :style="barFillStyle" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

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

// @vue/component
@Component
export default class VBar extends Vue {
    @Prop({ default: 0 })
    private readonly current: number;
    @Prop({ default: 0 })
    private readonly max: number;
    @Prop({ default: '#0068DC' })
    private readonly color: string;

    public get barFillStyle(): BarFillStyle {
        if (this.current > this.max) {
            return new BarFillStyle(this.color, '100%');
        }

        const width = (this.current / this.max) * 100 + '%';

        return new BarFillStyle(this.color, width);
    }
}
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

