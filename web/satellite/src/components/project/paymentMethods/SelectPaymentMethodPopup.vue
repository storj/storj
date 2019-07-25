// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-payment-popup-overflow" v-if="isPopupShown" v-on:keyup.enter="onDoneClick" v-on:keyup.esc="onCloseClick">
        <div class="add-payment-popup-container">
            <h1 class="add-payment-popup-container__title">Add Payment Method</h1>
            <PaymentMethodsSelector/>
            <div class="add-payment-popup-container__footer">
                <div class="add-payment-popup-container__footer__new-card-button" @click="onNewCardClick">
                    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M20 7.26316V3.87134C20 3.40351 19.7938 2.93567 19.4845 2.5848C19.1753 2.23392 18.7629 2 18.3505 2H1.64948C1.23711 2 0.824742 2.23392 0.515464 2.5848C0.206186 2.93567 0 3.40351 0 3.87134V7.26316H20Z" fill="#2683FF"/>
                        <path d="M0 9.36816V16.1852C0 16.5862 0.206186 16.9872 0.515464 17.288C0.824742 17.5887 1.23711 17.7892 1.64948 17.7892H18.3505C18.7629 17.7892 19.1753 17.5887 19.4845 17.288C19.7938 16.9872 20 16.5862 20 16.1852V9.36816H0ZM5.36083 15.1827H2.68041V13.8794H5.36083V15.1827ZM10.7217 15.1827H6.70103V13.8794H10.7217V15.1827Z" fill="#2683FF"/>
                    </svg>
                    <p class="add-payment-popup-container__footer__new-card-button__label" >+ New Card</p>
                </div>
                <Button
                        label="Done"
                        width="205px"
                        :onPress="onDoneClick"
                        height="48px" />
            </div>
            <div class="cross" @click="onCloseClick">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import Card from '@/components/project/CardChoiceItem.vue';
    import PaymentMethodsSelector from '@/components/project/paymentMethods/PaymentMethodsSelector.vue';
    import {
        APP_STATE_ACTIONS,
        NOTIFICATION_ACTIONS,
        PROJECT_PAYMENT_METHODS_ACTIONS
    } from '@/utils/constants/actionNames';

    @Component({
        components: {
            Button,
            Card,
            PaymentMethodsSelector
        }
    })

    export default class SelectPaymentMethodPopup extends Vue {

        public get isPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.isSelectPaymentMethodPopupShown;
        }

        public get defaultPaymentMethod(): PaymentMethod {
            return this.$store.state.userPaymentsMethodsModule.defaultPaymentMethod;
        }

        public onCloseClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SELECT_PAYMENT_METHOD_POPUP);
        }

        public async onDoneClick(): Promise<void> {
            const response = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.ATTACH, this.defaultPaymentMethod.id);
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Card successfully added');

            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SELECT_PAYMENT_METHOD_POPUP);
        }

        public onNewCardClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SELECT_PAYMENT_METHOD_POPUP);
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ATTACH_STRIPE_CARD_POPUP);
        }
    }
</script>

<style scoped lang="scss">
    h3 {
        font-family: 'font_regular';
        font-size: 16px;
        line-height: 23px;
        color: #2683ff;
        margin-right: 15px;
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
        width: 810px;
        height: 416px;
        background-color: white;
        border-radius: 6px;
        padding: 90px 102px 57px 88px;

        &__title {
             font-family: 'font_bold';
             font-size: 32px;
             line-height: 39px;
             color: #384B65;
             margin: 0;
        }

        &__chosen-card-container {
             display: flex;
             justify-content: space-between;
             align-items: center;
             width: 100%;
             margin-top: 60px;

            &__expand-container {
                display: flex;
            }
        }

        &__border {
             margin-top: 20px;
             width: 100%;
             height: 1px;
             background-color: rgba(169, 181, 193, 0.5);
        }

        &__expanded-area {
             width: 100%;
             height: 200px;
             overflow-y: auto;
        }

        &__footer {
             width: 100%;
             display: flex;
             margin-top: 10px;
             align-items: flex-start;
             justify-content: space-between;

            &__new-card-button {
                 height: 48px;
                 display: flex;
                 align-items: center;
                 cursor: pointer;

                 &__label {
                     margin-left: 20px;
                     font-family: 'font_bold';
                     font-size: 16px;
                     line-height: 23px;
                     color: #354049;
                }
            }
        }
    }

    .cross {
        position: absolute;
        top: 50px;
        right: 50px;
        width: 25px;
        height: 25px;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
    }

</style>
