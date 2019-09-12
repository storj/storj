// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container__card-container">
        <div class="payment-methods-container__card-container__info-area">
            <svg width="52" height="35" viewBox="0 0 52 35" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M31.1174 7.97095H20.1162V27.7581H31.1174V7.97095Z" fill="#FF5F00"/>
                <path d="M20.8148 17.8645C20.8148 13.8442 22.7007 10.2783 25.5994 7.97095C23.4691 6.29289 20.7799 5.27905 17.8462 5.27905C10.8963 5.27905 5.27344 10.9076 5.27344 17.8645C5.27344 24.8215 10.8963 30.45 17.8462 30.45C20.7799 30.45 23.4691 29.4362 25.5994 27.7581C22.7007 25.4858 20.8148 21.8849 20.8148 17.8645Z" fill="#EB001B"/>
                <path d="M45.9603 17.8645C45.9603 24.8215 40.3375 30.45 33.3875 30.45C30.4539 30.45 27.7647 29.4362 25.6343 27.7581C28.5679 25.4508 30.4189 21.8849 30.4189 17.8645C30.4189 13.8442 28.533 10.2783 25.6343 7.97095C27.7647 6.29289 30.4539 5.27905 33.3875 5.27905C40.3375 5.27905 45.9603 10.9425 45.9603 17.8645Z" fill="#F79E1B"/>
            </svg>
            <div class="payment-methods-container__card-container__info-area__info-container">
                <h1>**** **** **** {{1111}}</h1>
            </div>
            <div class="payment-methods-container__card-container__info-area__expire-container">
                <h2>Expires</h2>
                <h1>{{1}}/{{1111}}</h1>
            </div>
        </div>
        <div class="payment-methods-container__card-container__button-area">
            <div class="payment-methods-container__card-container__default-button" v-if="false">
                <p class="payment-methods-container__card-container__default-button__label">Default</p>
            </div>
            <div class="payment-methods-container__card-container__dots-container">
                <svg width="12" height="4" viewBox="0 0 12 4" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="4" height="4" rx="2" fill="#354049"/>
                    <rect x="8" width="4" height="4" rx="2" fill="#354049"/>
                </svg>
                <div class="dialog">

                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';

    @Component({
        components: {
            // MakeDefaultPaymentMethodDialog,
            Button,
            // DeletePaymentMethodDialog,
        }
    })
    export default class CardComponent extends Vue {}
</script>

<style scoped lang="scss">
    h1 {
        font-family: 'font_bold';
        font-size: 16px;
        line-height: 21px;
        color: #61666B;
        margin-block-start: 0.5em;
        margin-block-end: 0.5em;
    }

    h2 {
        font-family: 'font_regular';
        font-size: 16px;
        line-height: 21px;
        color: #61666B;
        margin-right: 5px;
    }

    .payment-methods-container__card-container {
        width: calc(100% - 40px);
        margin-top: 12px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 20px;
        background-color: #F5F6FA;
        border-radius: 6px;

        &__info-area {
            width: 75%;
            display: flex;
            align-items: center;
            justify-content: flex-start;

            &__card-logo {
                height: 70px;
                width: 85px;
            }

            &__info-container {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                width: auto;
                margin-left: 15px;
            }

            &__expire-container {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                width: auto;
                margin-left: 30px;
            }
        }

        &__button-area {
            display: flex;
            justify-content: center;
            align-items: center;
        }

        &__default-button {
            width: 100px;
            height: 34px;
            border-radius: 6px;
            background-color: white;
            display: flex;
            justify-content: center;
            align-items: center;

            &__label {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 23px;
                color: #000;
            }
        }

        &__dots-container {
            width: 20px;
            height: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
            margin-left: 10px;
            cursor: pointer;
            position: relative;
        }
    }

    .dialog {
        position: absolute;
        top: 15px;
        right: -50px;
        z-index: 100;
        background-image: url("../../../../static/images/payments/Dialog.svg");
        background-size: contain;
        width: 122px;
        height: 167px;
    }
</style>
