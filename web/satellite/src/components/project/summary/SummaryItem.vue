// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="summary-item" :style="{ backgroundColor: backgroundColor }">
        <p class="summary-item__title" :style="{ color: titleColor }">{{ title }}</p>
        <p v-if="isMoney" class="summary-item__value" :style="{ color: valueColor }">{{ value | centsToDollars }}</p>
        <p v-else class="summary-item__value" :style="{ color: valueColor }">{{ value }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class SummaryItem extends Vue {
    @Prop({ default: '#fff' })
    public readonly backgroundColor: string;
    @Prop({ default: '#1b2533' })
    public readonly titleColor: string;
    @Prop({ default: '#000' })
    public readonly valueColor: string;
    @Prop({ default: '' })
    public readonly title: string;
    @Prop({ default: 0 })
    public readonly value: number;
    @Prop({ default: false })
    public readonly isMoney: boolean;
}
</script>

<style scoped lang="scss">
    .summary-item {
        padding: 25px 30px;
        width: 100%;
        border-radius: 6px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 14px;
            margin: 0 0 12px;
        }

        &__value {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 23px;
            letter-spacing: -1px;
            margin: 0;
        }
    }
</style>
