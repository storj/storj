// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-container" :style="{ width }">
        <p class="info-container__label">{{ label }}</p>
        <p class="info-container__value">{{ value }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

@Component
export default class SingleInfo extends Vue {
    @Prop({default: '100%'})
    public readonly width: string;
    @Prop({default: 'Label'})
    public readonly label: string;
    @Prop({default: 'value'})
    public readonly value: string;
}
</script>

<style scoped lang="scss">
    .info-container {
        width: 45%;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        background: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        box-sizing: border-box;
        border-radius: 10px;
        padding: 20px 30px;

        &__label,
        &__value {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--regular-text-color);
        }

        &__value {
            font-family: 'font_bold', sans-serif;
            font-size: 20px;
        }
    }
</style>
