// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-payment-popup-overflow" v-on:keyup.enter="onDoneClick" v-on:keyup.esc="onCloseClick">
        <div class="add-payment-popup-container">
            <h1 class="add-payment-popup-container__title">Add Payment Method</h1>
            <form action="/charge" method="post" id="payment-form">
                <div class="form-row">
                    <div id="card-element">
                        <!-- A Stripe Element will be inserted here. -->
                    </div>

                    <!-- Used to display form errors. -->
                    <div class="cross" @click="onCloseClick">
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z"
                                  fill="#384B65"/>
                        </svg>
                    </div>
                    <div id="card-errors" role="alert"></div>
                </div>
                <button
                        width="140px"
                        height="48px"
                        label="Submit"
                        :on-press="onSubmitClick"
                >Submit</button>

            </form>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from "vue-property-decorator";
    import Button from "@/components/common/Button.vue";
    import {
        APP_STATE_ACTIONS,
        NOTIFICATION_ACTIONS,
        PROJECT_PAYMENT_METHODS_ACTIONS
    } from "@/utils/constants/actionNames";
    // import Card from "@/components/project/CardChoiceItem.vue";

    @Component(
        {
            data: function(){
                return{
                }
            },

            mounted: function () {
                console.log("test");
                if (!window["Stripe"]) {
                    console.log("stripe v3 library not loaded!");

                    return;
                }
                const stripe = window["Stripe"]("pk_test_bXKJTU49iu1dy9Al0iEqlfVc00Ze0m5lXT");
                console.log(stripe)
                if (!stripe) {
                    console.log("unable to initialize stripe");

                    return;
                }

                const elements = stripe.elements();
                if (!elements) {
                    console.log("Unable to instantiate elements");

                    return;
                }

                const card = elements.create("card");
                if (!card) {
                    console.log("unable to create card");

                    return;
                }

                card.mount("#card-element");

                card.addEventListener("change", function (event) {
                    const displayError = document.getElementById("card-errors") as HTMLElement;
                    if (event.error) {
                        displayError.textContent = event.error.message;
                    } else {
                        displayError.textContent = "";
                    }
                });

                const form = document.getElementById("payment-form") as HTMLElement;
                let self = this;
                form.addEventListener("submit", function (event) {
                    event.preventDefault();
                    console.log("beforeCreate");
                    stripe.createToken(card).then(async function (result: any) {
                        console.log("inside create callback")
                        console.log(result)
                        if(result.token.card.funding == 'prepaid') {
                            self.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Prepaid cards not supported')
                            return;
                        }
                        const response = await self.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.ADD, result.token.id);
                        console.log(response);
                        if (!response.isSuccess){
                            self.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.error);
                        }

                        await self.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
                        self.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Card successfully added');
                        self.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ADD_NEW_PAYMENT_METHOD_POPUP);
                    });
                });
            },

            computed: {
                isPopupShown: function () {
                    return this.$store.state.appStateModule.appState.isAddNewPaymentMethodPopupShown;
                }
            },

            methods: {
                test: function () {
                },
                onSubmitClick: function () {
                    console.log("submit click");
                },
                onCloseClick: function () {
                    this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ADD_NEW_PAYMENT_METHOD_POPUP);
                }

            },

            components: {
                Button,
                // Card,
            }
        }
    )

    export default class AddNewPaymentMethodPopup extends Vue {
    }
</script>

<style scoped lang="scss">
    .StripeElement {
        box-sizing: border-box;

        height: 40px;

        padding: 10px 12px;

        border: 1px solid transparent;
        border-radius: 4px;
        background-color: white;

        box-shadow: 0 1px 3px 0 #e6ebf1;
        -webkit-transition: box-shadow 150ms ease;
        transition: box-shadow 150ms ease;
    }

    .StripeElement--focus {
        box-shadow: 0 1px 3px 0 #cfd7df;
    }

    .StripeElement--invalid {
        border-color: #fa755a;
    }

    .StripeElement--webkit-autofill {
        background-color: #fefde5 !important;
    }

    .add-payment-popup-overflow {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1121;
        display: flex;
        justify-content: center;
        align-items: center;
    }

    .add-payment-popup-container {
        position: relative;
        width: 541px;
        height: 678px;
        background-color: white;
        border-radius: 6px;
        padding: 38px 30px;

        &__title {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #384B65;
            margin: 0;
        }

    }

    .cross {
        position: absolute;
        top: 39px;
        right: 39px;
        width: 25px;
        height: 25px;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
    }
</style>