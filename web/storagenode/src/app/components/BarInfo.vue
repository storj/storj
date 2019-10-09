// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="remaining-space-container">
        <p class="remaining-space-container__title">{{label}}</p>
        <p class="remaining-space-container__amount"><b>{{remaining}}</b></p>
        <div class="remaining-space-container__bar">
            <VInfo :text="infoMessage">
                <VBar
                    :current="currentBarAmount"
                    :max="maxBarAmount"
                    color="#224CA5"
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
    .remaining-space-container {
        width: 325px;
        height: 90px;
        background-color: #FFFFFF;
        border: 1px solid #E9EFF4;
        border-radius: 11px;
        padding: 32px 40px;
        margin-bottom: 13px;
        position: relative;

        &__title {
            margin: 0;
            font-size: 14px;
            line-height: 21px;
            color: #586C86;
        }

        &__amount {
            margin: 0;
            font-size: 32px;
            line-height: 57px;
            color: #535F77;
        }
    }
</style>
