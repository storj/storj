// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="remaining-space-container">
        <p class="remaining-space-container__title">{{label}}</p>
        <p class="remaining-space-container__amount"><b>{{remaining}}GB</b></p>
        <div class="remaining-space-container__bar">
            <InfoComponent :text="infoMessage">
                <Bar :current="currentBarAmount" :max="maxBarAmount" color="#224CA5"/>
            </InfoComponent>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import Bar from '@/app/components/Bar.vue';
import InfoComponent from '@/app/components/InfoComponent.vue';

@Component ({
    components: {
        Bar,
        InfoComponent,
    },
})
export default class BarInfoContainer extends Vue {
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
        return this.amount.toFixed(2);
    }
}
</script>

<style lang="scss">
    .remaining-space-container {
        width: 325px;
        height: 90px;
        background-color: #FFFFFF;
        border: 1px solid #E9EFF4;
        border-radius: 11px;
        padding: 34px 36px 39px 39px;
        margin-bottom: 32px;
        position: relative;

        &__title {
            font-size: 14px;
            color: #586C86;
        }

        &__amount {
            font-size: 32px;
            line-height: 57px;
            color: #535F77;
        }
    }
</style>
