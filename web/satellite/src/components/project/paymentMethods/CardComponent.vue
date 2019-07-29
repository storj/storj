// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container__card-container">
        <div class="payment-methods-container__card-container__info-area">
            <img class="payment-methods-container__card-container__info-area__card-logo" src="@/../static/images/Logo.svg">
            <div class="payment-methods-container__card-container__info-area__info-container">
                <h1>xxxx {{paymentMethod.lastFour}}</h1>
            </div>
            <div class="payment-methods-container__card-container__info-area__expire-container">
                <h2>Expires</h2>
                <h1>{{paymentMethod.expMonth}}/{{paymentMethod.expYear}}</h1>
            </div>
            <h3 class="payment-methods-container__card-container__info-area__added-text">Added on {{formatDate(paymentMethod.addedAt)}}</h3>
        </div>
        <div class="payment-methods-container__card-container__default-button" v-if="paymentMethod.isDefault">
            <p class="payment-methods-container__card-container__default-button__label">Default</p>
        </div>
        <div class="payment-methods-container__card-container__make-default-container" v-if="!paymentMethod.isDefault">
            <div class="payment-methods-container__card-container__make-default-container__make-button" v-on:click="onMakeDefaultClick(paymentMethod.id)" id="makeDefaultPaymentMethodButton">
                <p class="payment-methods-container__card-container__make-default-container__make-button__label">Make Default</p>
            </div>
            <MakeDefaultPaymentMethodDialog :paymentMethodID="paymentMethod.id" v-if="isSetDefaultPaymentMethodPopupShown"/>
        </div>
        <div class="payment-methods-container__card-container__delete-button-container">
            <div class="payment-methods-container__card-container__delete-button-container__delete-button" v-on:click="onDeletePaymentMethodClick" id="deletePaymentMethodButton">
                <svg width="34"
                    height="34"
                    viewBox="0 0 34 34"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg">
                    <rect width="34" height="34" rx="17" fill="#EB5757"/>
                    <path d="M19.7834 11.9727V11.409C19.7834 10.6576 19.1215 10 18.2706 10H16.0014C15.1504 10 14.4886 10.6576 14.4886 11.409V11.9727H10.7065V13.1938H12.0302V22.3057C12.0302 23.5269 12.9758 24.4662 14.0158 24.4662H20.1616C21.2962 24.4662 22.1471 23.5269 22.1471 22.3057V13.1938H23.4709V11.9727H19.7834ZM16.6632 22.3057H15.3395V14.2271H16.6632V22.3057ZM18.9324 22.3057H17.6087V14.2271H18.9324V22.3057Z" fill="white"/>
                </svg>
            </div>
            <DeletePaymentMethodDialog :paymentMethodID="paymentMethod.id" v-if="isDeletePaymentMethodPopupShown"/>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import { APP_STATE_ACTIONS, } from '@/utils/constants/actionNames';
    import DeletePaymentMethodDialog from '@/components/project/paymentMethods/DeletePaymentMethodDialog.vue';
    import MakeDefaultPaymentMethodDialog from '@/components/project/paymentMethods/MakeDefaultPaymentMethodDialog.vue';

    @Component({
        components: {
            MakeDefaultPaymentMethodDialog,
            Button,
            DeletePaymentMethodDialog,
        }
    })
    export default class CardComponent extends Vue {
        @Prop({default: {}})
        private readonly paymentMethod: PaymentMethod;

        public formatDate(d: string): string {
            return new Date(d).toLocaleDateString('en-US', {timeZone: 'UTC'});
        }

        public async onMakeDefaultClick(): Promise<void> {
            if (this.getSetDefaultPaymentMethodID == this.paymentMethod.id) {
                this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);

                return;
            }

            this.$store.dispatch(APP_STATE_ACTIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP, this.paymentMethod.id);
        }

        public async onDeletePaymentMethodClick(): Promise<void> {
            if (this.getDeletePaymentMethodID == this.paymentMethod.id) {
                this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);

                return;
            }

            this.$store.dispatch(APP_STATE_ACTIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP, this.paymentMethod.id);
        }

        public get getDeletePaymentMethodID(): string {
            return this.$store.state.appStateModule.appState.deletePaymentMethodID;
        }

        public get getSetDefaultPaymentMethodID(): string {
            return this.$store.state.appStateModule.appState.setDefaultPaymentMethodID;
        }

        public get isDeletePaymentMethodPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.deletePaymentMethodID == this.paymentMethod.id;
        }

        public get isSetDefaultPaymentMethodPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.setDefaultPaymentMethodID == this.paymentMethod.id;
        }
    }
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
        width: calc(100% - 80px);
        margin-top: 24px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 25px 40px 25px 40px;
        background-color: white;
        border-radius: 6px;

        &__info-area {
            width: 75%;
            display: flex;
            align-items: center;
            justify-content: space-between;

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
            }

            &__expire-container {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                width: auto;
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

        &__make-default-container {
            position: relative;

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
        }

        &__delete-button-container {
            position: relative;

            &__delete-button {

                svg {
                    cursor: pointer;
                }
            }
        }
    }
</style>