// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bar-info-container">
        <p class="bar-info-container__title">{{ label }}</p>
        <p class="bar-info-container__amount"><b>{{ remaining }}</b></p>
        <div class="bar-info-container__bar">
            <VInfo :text="infoMessage">
                <VBar
                    :current="currentBarAmount"
                    :max="maxBarAmount"
                />
            </VInfo>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VBar from '@/app/components/VBar.vue';
import VInfo from '@/app/components/VInfo.vue';

import { formatBytes } from '@/app/utils/converter';

@Component ({
    components: {
        VBar,
        VInfo,
    },
})
export default class BarInfo extends Vue {
    @Prop({default: ''})
    private readonly label: string;
    @Prop({default: ''})
    private readonly amount: number;
    @Prop({default: ''})
    private readonly infoText: string;
    @Prop({default: ''})
    private readonly currentBarAmount: number;
    @Prop({default: ''})
    private readonly maxBarAmount: number;

    public get infoMessage(): string {
        return `${Math.floor(100 - (this.currentBarAmount / this.maxBarAmount) * 100)}% ${this.infoText}`;
    }

    public get remaining(): string {
        return formatBytes(this.amount);
    }
}
</script>

<style scoped lang="scss">
    p {
        margin: 0;
    }

    .bar-info-container {
        width: calc(100% - 60px);
        height: 99px;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 11px;
        padding: 32px 30px;
        margin-bottom: 13px;
        position: relative;

        &__title {
            font-size: 14px;
            line-height: 21px;
            color: var(--title-text-color);
        }

        &__amount {
            font-size: 32px;
            line-height: 57px;
            color: var(--regular-text-color);
        }
    }
</style>
