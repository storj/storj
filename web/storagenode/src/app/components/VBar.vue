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
    width: string;

    public constructor(width: string) {
        this.width = width;
    }
}

@Component
export default class VBar extends Vue {
    @Prop({default: ''})
    private readonly current: string;
    @Prop({default: ''})
    private readonly max: string;

    public get barFillStyle(): BarFillStyle {
        const width = (parseFloat(this.current) / parseFloat(this.max)) * 100 + '%';

        return new BarFillStyle(width);
    }
}
</script>

<style scoped lang="scss">
    .bar-container {
        width: 100%;
        height: 8px;
        margin-top: 10px;
        border-radius: 4px;
        background-color: var(--bar-background-color);
        position: relative;

        &__fill {
            height: 100%;
            position: absolute;
            left: 0;
            top: 0;
            border-radius: 20px;
            background-color: var(--navigation-link-color);
        }
    }
</style>

