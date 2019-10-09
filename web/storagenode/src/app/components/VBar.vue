// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bar-container">
        <div class="bar-container__fill" :style="barFillStyle"></div>
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

@Component
export default class VBar extends Vue {
    @Prop({default: ''})
    private readonly current: string;
    @Prop({default: ''})
    private readonly max: string;
    @Prop({default: '#224CA5'})
    private readonly color: string;

    public get barFillStyle(): BarFillStyle {
        const width = (parseFloat(this.current) / parseFloat(this.max)) * 100 + '%';

        return new BarFillStyle(this.color, width);
    }
}
</script>

<style scoped lang="scss">
    .bar-container {
        width: 100%;
        height: 8px;
        margin-top: 5px;
        border-radius: 4px;
        background-color: #F4F6F9;
        position: relative;

        &__fill {
            height: 100%;
            position: absolute;
            left: 0;
            top: 0;
            border-radius: 20px;
        }
    }
</style>

