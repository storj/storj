// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="remaining-space-container">
        <p class="remaining-space-container__title">{{label}}</p>
        <p class="remaining-space-container__amount"><b>{{amount}}</b></p>
        <div class="remaining-space-container__bar">
            <InfoComponent :text="infoMessage">
                <template>
                    <Bar :current="currentBarAmount" :max="maxBarAmount" color="#224CA5"/>
                </template>
            </InfoComponent>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Bar from '@/components/Bar.vue';
    import InfoComponent from '@/components/InfoComponent.vue';

    @Component ({
        props: {
            label: String,
            amount: String,
            infoText: String,
            currentBarAmount: String,
            maxBarAmount: String,
        },
        computed: {
            infoMessage: function (): string {
                return `${100 - Math.round((parseFloat(this.$props.currentBarAmount)/parseFloat(this.$props.maxBarAmount))*100)}% ${this.$props.infoText}`
            }
        },

        components: {
            Bar,
            InfoComponent,
        },
    })

    export default class BarInfoContainer extends Vue {
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
