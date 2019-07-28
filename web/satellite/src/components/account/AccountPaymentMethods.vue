// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container">
        <div class="payment-methods-container__card-container">
            <div class="payment-methods-container__card-container__info-area">
                <img class="payment-methods-container__card-container__info-area__card-logo" src="../../../static/images/Logo.svg">
                <div class="payment-methods-container__card-container__info-area__info-container">
                    <h1>xxxx 0000</h1>
                    <h2>Shawn Wilkinson</h2>
                </div>
                <div class="payment-methods-container__card-container__info-area__expire-container">
                    <h2>Expires</h2>
                    <h1>12/2020</h1>
                </div>
                <h3 class="payment-methods-container__card-container__info-area__added-text">Added on 29 May 2019</h3>
            </div>
            <div class="payment-methods-container__card-container__default-button" v-if="true">
                <p class="payment-methods-container__card-container__default-button__label">Default</p>
            </div>
            <div class="payment-methods-container__card-container__button-area" v-if="false">
                <div class="payment-methods-container__card-container__button-area__make-button">
                    <p class="payment-methods-container__card-container__button-area__make-button__label">Make Default</p>
                </div>
                <svg class="payment-methods-container__card-container__button-area__delete-button"
                    width="34"
                    height="34"
                    viewBox="0 0 34 34"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg">
                        <rect width="34" height="34" rx="17" fill="#EB5757"/>
                        <path d="M19.7834 11.9727V11.409C19.7834 10.6576 19.1215 10 18.2706 10H16.0014C15.1504 10 14.4886 10.6576 14.4886 11.409V11.9727H10.7065V13.1938H12.0302V22.3057C12.0302 23.5269 12.9758 24.4662 14.0158 24.4662H20.1616C21.2962 24.4662 22.1471 23.5269 22.1471 22.3057V13.1938H23.4709V11.9727H19.7834ZM16.6632 22.3057H15.3395V14.2271H16.6632V22.3057ZM18.9324 22.3057H17.6087V14.2271H18.9324V22.3057Z" fill="white"/>
                </svg>
            </div>
        </div>
        <Button
            class="payment-methods-container__add-button"
            label="Add Card"
            width="140px"
            height="48px"
            isDisabled="true" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';

    @Component({
        components: {
            Button,
        }
    })
    export default class AccountPaymentMethods extends Vue {}
</script>

<style scoped lang="scss">
    .payment-methods-container {
        margin-top: 83px;
        
        &__card-container {
            width: calc(100% - 80px);
            margin-top: 37px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 50px 40px 50px 40px;
            background-color: white;
            border-radius: 6px;
    
            &__info-area {
                width: 75%;
                display: flex;
                align-items: center;
                justify-content: space-between;
                
                &__card-logo {
                    height: 50px;
                    width: 50px;
                }
                
                &__info-container {
                
                    h1 {
                        font-family: 'font_bold';
                        font-size: 16px;
                        line-height: 21px;
                        color: #61666B;
                    }
                    
                    h2 {
                        font-family: 'font_regular';
                        font-size: 16px;
                        line-height: 21px;
                        color: #61666B;
                        margin-block-start: 0.5em;
                        margin-block-end: 0.5em;
                    }
                }
                
                &__expire-container {
                    
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
                    }
                }
                
                &__added-text {
                    font-family: 'font_regular';
                    font-size: 16px;
                    line-height: 21px;
                    color: #61666B;
                }
            }
            
            &__default-button {
                width: 100px;
                height: 34px;
                border-radius: 6px;
                background-color: #F5F6FA;
                display: flex;
                justify-content: center;
                align-items: center;
            
                &__label {
                    font-family: 'font_medium';
                    font-size: 16px;
                    line-height: 23px;
                    color: #AFB7C1;
                }
            }
            
            &__button-area {
                width: 20%;
                display: flex;
                justify-content: space-between;
                align-items: center;
                
                &__make-button {
                    width: 134px;
                    height: 34px;
                    border-radius: 6px;
                    background-color: #DFEDFF;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    cursor: pointer;
                    
                    &__label {
                        font-family: 'font_medium';
                        font-size: 16px;
                        line-height: 23px;
                        color: #2683FF;
                    }
                }
                
                svg {
                    cursor: pointer;
                }
            }
        }
    
        &__add-button {
            margin-top: 35px;
        }
    }
</style>
