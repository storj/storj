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
        PROJECT_PAYMENT_METHODS_ACTIONS,
        USER_PAYMENT_METHODS_ACTIONS
    } from '@/utils/constants/actionNames';
    import Checkbox from '@/components/common/Checkbox.vue';
    import { setupStripe } from '@/utils/stripeHelper';

    @Component(
        {
            components: {
                Button,
                Checkbox,
            }
        }
    )

    export default class NewProjectPaymentMethodComponent extends Vue {
        private makeDefault: boolean = false;
        private isSaveButtonEnabled: boolean = true;

        public mounted(): void {
            setupStripe(this, async result => {
                const input = {
                    token: result.token.id,
                    makeDefault: this.makeDefault} as AddPaymentMethodInput;

                const response = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.ADD, input);
                this.isSaveButtonEnabled = true;
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                    return;
                }
                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Card successfully added');

                const projectPaymentsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
                if (!projectPaymentsResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch payment methods: ' + projectPaymentsResponse.errorMessage);
                }

                const userPaymentMethodResponse = await this.$store.dispatch(USER_PAYMENT_METHODS_ACTIONS.FETCH);
                if (!userPaymentMethodResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch user payment methods: ' + userPaymentMethodResponse.errorMessage);
                }
            });
        }

        public get projectPaymentMethodsCount(): number {
            if (this.$store.state.projectPaymentsMethodsModule.paymentMethods) {
                return this.$store.state.projectPaymentsMethodsModule.paymentMethods.length;
            }

            return 0;
        }

        public toggleMakeDefault(value: boolean): void {
            this.makeDefault = value;
        }

        public onSaveClick(): void {
            if (!this.isSaveButtonEnabled) {
                return;
            }

            const form = document.getElementById('payment-form') as HTMLElement;
            const saveEvent = new CustomEvent('submit', {'bubbles': true});
            form.dispatchEvent(saveEvent);

            this.isSaveButtonEnabled = false;
        }
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