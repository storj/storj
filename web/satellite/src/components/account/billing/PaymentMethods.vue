// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-area">
        <h1 class="payment-methods-area__title">Payment Method</h1>
        <div class="payment-methods-area__balance-area">
            <Button
                class="button"
                label="Add STORJ"
                width="123px"
                height="48px"
                :onPress="onAddSTORJ"/>
            <Button
                class="button"
                label="Add Card"
                width="123px"
                height="48px"
                :onPress="onAddCard"/>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';

    @Component({
        components: {
            Button
        }
    })
    export default class PaymentMethods extends Vue {
        public onAddSTORJ(): void {
            return;
        }
        public onAddCard(): void {
            return;
        }
    }
</script>

<style scoped lang="scss">
    h1,
    span {
        margin: 0;
        color: #354049;
    }

    .button {

        &:hover {
            background-color: #0059D0;
            box-shadow: none;
        }
    }

    .payment-methods-area {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 40px;
        margin-bottom: 47px;
        background-color: #FFFFFF;
        border-radius: 8px;
        font-family: 'font_regular';

        &__title {
            font-family: 'font_bold';
            font-size: 32px;
            line-height: 48px;
        }

        &__balance-area {
            display: flex;
            align-items: center;

            &__balance {
                margin-right: 27px;
                font-size: 18px;
                color: rgba(53, 64, 73, 0.5);

                b {
                    color: #354049;
                }
            }
        }
    }
</style>
