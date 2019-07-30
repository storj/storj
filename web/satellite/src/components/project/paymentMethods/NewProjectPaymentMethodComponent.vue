// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-payment-popup-overflow" v-on:keyup.enter="onDoneClick" v-on:keyup.esc="onCloseClick">
        <div class="add-payment-popup-container">
            <div class="card-form-input">
                <img src="../../../../static/images/Card.svg"/>
                <div id="payment-form">
                    <div class="stripe">
                        <StripeInput class="stripe-input-container"
                                     :onStripeResponseCallback="onStripeResponse"
                        />
                    </div>
                    <div class="submit-container">
                        <div class="checkbox-container" v-if="projectPaymentMethodsCount > 0">
                            <Checkbox @setData="toggleMakeDefault"/>
                            <h2>Make Default</h2>
                        </div>
                        <Button
                                label="Save"
                                width="135px"
                                height="48px"
                                :on-press="onSaveClick"/>
                    </div>

                </div>
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
    import { AddPaymentMethodInput } from '@/types/invoices';
    import StripeInput from '@/components/common/StripeInput.vue';

    @Component(
        {
            components: {
                Button,
                Checkbox,
                StripeInput
            }
        }
    )

    export default class NewProjectPaymentMethodComponent extends Vue {
        private makeDefault: boolean = false;
        private isSaveButtonEnabled: boolean = true;

        public async onStripeResponse(result: any) {
            const input:AddPaymentMethodInput = new AddPaymentMethodInput(result.token.id, this.makeDefault);

            const response = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.ADD, input);
            this.isSaveButtonEnabled = true;
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                return;
            }
            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Card successfully added');

            const projectPaymentsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
            if (!projectPaymentsResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch payment methods: ${projectPaymentsResponse.errorMessage}`);
            }

            const userPaymentMethodResponse = await this.$store.dispatch(USER_PAYMENT_METHODS_ACTIONS.FETCH);
            if (!userPaymentMethodResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch user payment methods: ${userPaymentMethodResponse.errorMessage}`);
            }
        }

        public get projectPaymentMethodsCount(): number {
            return this.$store.state.projectPaymentsMethodsModule.paymentMethods.length;
        }

        public toggleMakeDefault(value: boolean): void {
            this.makeDefault = value;
        }

        public onSaveClick(): void {
            if (!this.isSaveButtonEnabled) {
                return;
            }

            this.$emit('onSubmitStripeInputEvent');

            this.isSaveButtonEnabled = false;
        }
    }
</script>

<style scoped lang="scss">
    .card-form-input {
        display: flex;
        justify-content: center;
        align-items: center;
        width: 100%;

        #payment-form {
            display: flex;
            width: 100%;
            justify-content: space-between;
            align-items: center;

            .submit-container {
                display: flex;
                min-width: 290px;
                align-items: center;
                justify-content: space-between;
                margin-left: 41px;
            }
        }

        .stripe {
            width: 100%;
            align-items: center;
        }

        img {
            margin-top: 7px;
            margin-right: 25px;
            margin-left: -20px;
        }

    }

    .stripe-input-container {
        width: 100%;
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
            margin-left: 5px;
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