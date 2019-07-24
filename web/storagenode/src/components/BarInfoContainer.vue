// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="remaining-space-container">
        <p class="remaining-space-container__title">{{label}}</p>
        <p class="remaining-space-container__amount"><b>{{amount}}</b></p>
        <div class="remaining-space-container__bar">
            <InfoComponent :text="infoMessage">
                <Bar :current="currentBarAmount" :max="maxBarAmount" color="#224CA5"/>
            </InfoComponent>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import Bar from '@/components/Bar.vue';
    import InfoComponent from '@/components/InfoComponent.vue';

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
        private readonly amount: string;
        @Prop({default: ''})
        private readonly infoText: string;
        @Prop({default: ''})
        private readonly currentBarAmount: string;
        @Prop({default: ''})
        private readonly maxBarAmount: string;

        public get infoMessage(): string {
            return `${100 - Math.round((parseFloat(this.currentBarAmount) / parseFloat(this.maxBarAmount)) * 100)}% ${this.infoText}`;
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
