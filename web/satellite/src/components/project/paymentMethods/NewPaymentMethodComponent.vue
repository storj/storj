// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-payment-popup-overflow" v-on:keyup.enter="onDoneClick" v-on:keyup.esc="onCloseClick">
        <div class="add-payment-popup-container">
            <div class="card-form-input">
                <img src="../../../../static/images/Card.svg"/>
                <form id="payment-form">
                    <div class="form-row">
                        <div id="card-element">
                            <!-- A Stripe Element will be inserted here. -->
                        </div>

                        <!-- Used to display form errors. -->
                        <div id="card-errors" role="alert"></div>
                    </div>
                    <div class="checkbox-container" v-if="projectPaymentMethodsCount > 0">
                        <Checkbox @setData="toggleMakeDefault"/>
                        <h2>Make Default</h2>
                    </div>
                    <Button
                            label="Save"
                            width="135px"
                            height="48px"
                            :on-press="onSaveClick"/>
                </form>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import {
        NOTIFICATION_ACTIONS,
        PROJECT_PAYMENT_METHODS_ACTIONS
    } from '@/utils/constants/actionNames';
    import Checkbox from '@/components/common/Checkbox.vue';

    @Component(
        {
            data: function () {
                return {
                    makeDefault: false,
                };
            },
            mounted: function () {
                if (!window['Stripe']) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Stripe library not loaded');

                    return;
                }

                const stripe = window['Stripe'](process.env.VUE_APP_STRIPE_PUBLIC_KEY);
                if (!stripe) {
                    console.error('Unable to initialize stripe');
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to initialize stripe');

                    return;
                }

                const elements = stripe.elements();
                if (!elements) {
                    console.error('Unable to instantiate elements');
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to instantiate elements');

                    return;
                }

                const card = elements.create('card');
                if (!card) {
                    console.error('Unable to create card');
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to create card');

                    return;
                }

                card.mount('#card-element');

                card.addEventListener('change', function (event) {
                    const displayError = document.getElementById('card-errors') as HTMLElement;
                    if (event.error) {
                        displayError.textContent = event.error.message;
                    } else {
                        displayError.textContent = '';
                    }
                });

                const form = document.getElementById('payment-form') as HTMLElement;
                let self = this;
                form.addEventListener('submit', function (event) {
                    event.preventDefault();
                    stripe.createToken(card).then(async function (result: any) {
                        if (result.token.card.funding == 'prepaid') {
                            self.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Prepaid cards not supported');

                            return;
                        }

                        const input = {
                            token: result.token.id,
                            makeDefault: self.$data.makeDefault} as AddPaymentMethodInput;

                        const response = await self.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.ADD, input);
                        if (!response.isSuccess) {
                            self.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.error);
                        }

                        await self.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
                        self.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Card successfully added');
                        card.clear();
                    });
                });
            },

            computed: {
                projectPaymentMethodsCount: function () {
                    if (this.$store.state.projectPaymentsMethodsModule.paymentMethods) {
                        return this.$store.state.projectPaymentsMethodsModule.paymentMethods.length;
                    } else {
                        return 0;
                    }
                }
            },

            methods: {
                toggleMakeDefault: function (value: boolean) {
                    this.$data.makeDefault = value;
                },
                onSaveClick: function () {
                    const form = document.getElementById('payment-form') as HTMLElement;
                    const saveEvent = new CustomEvent('submit', {'bubbles': true});
                    form.dispatchEvent(saveEvent);
                }
            },

            components: {
                Button,
                Checkbox,
            }
        }
    )

    export default class AddNewPaymentMethodPopup extends Vue {
    }
</script>

<style scoped lang="scss">
    .StripeElement {
        box-sizing: border-box;

        width: 484px;

        padding: 13px 12px;

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

    .card-form-input {
        display: flex;
        justify-content: center;
        align-items: center;
        width: 100%;

        form {
            display: flex;
            width: 100%;
            justify-content: space-between;
            align-items: center;
        }

        img {
            margin-top: 7px;
            margin-right: 25px;
            margin-left: -20px;
        }
    }

    .checkbox-container {
        display: flex;
        justify-content: center;
        align-items: center;

        h2 {
            font-family: 'font_regular';
            font-size: 12px;
            line-height: 18px;
            color: #384B65;
        }

    }

    .add-payment-popup-overflow {
        margin-top: 37px;
    }

    .add-payment-popup-container {
        width: calc(100% - 80px);
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 25px 40px 25px 40px;
        background-color: white;
        border-radius: 6px;

    }

</style>