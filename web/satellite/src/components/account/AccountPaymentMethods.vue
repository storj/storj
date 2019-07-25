// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container">
        <div v-for="method in paymentMethods" class="payment-methods-container__card-container">
            <CardComponent :paymentMethod="method"/>
        </div>
        <Button
            class="payment-methods-container__add-button"
            label="Add Card"
            width="140px"
            height="48px"
            :onPress="onNewCardClick"
        />
        <NewUserPaymentMethodPopup />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import CardComponent from '@/components/project/paymentMethods/CardComponent.vue';
    import NewUserPaymentMethodPopup from '@/components/project/paymentMethods/NewUserPaymentMethodPopup.vue';
    import Button from '@/components/common/Button.vue';
    import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            NewUserPaymentMethodPopup,
            CardComponent,
            Button
        }
    })
    export default class AccountPaymentMethods extends Vue {

        public get paymentMethods(): PaymentMethod[] {
            return this.$store.state.userPaymentsMethodsModule.userPaymentMethods;
        }

        public onNewCardClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ADD_USER_PAYMENT_POPUP);
        }
    }
</script>

<style scoped lang="scss">
    .payment-methods-container {
        margin-top: 83px;

        &__add-button {
            margin-top: 20px;
        }
    }

    @media screen and (max-height: 1280px) {
        .payment-methods-container {
            height: 1000px;
            overflow-y: scroll;

            &::-webkit-scrollbar,
            &::-webkit-scrollbar-track,
            &::-webkit-scrollbar-thumb {
                 visibility: hidden;
            }
        }
    }

    @media screen and (max-height: 1240px) {
        .payment-methods-container {
            height: 960px;
            overflow-y: scroll;
        }
    }


    @media screen and (max-height: 1200px) {
        .payment-methods-container {
            height: 920px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 1160px) {
        .payment-methods-container {
            height: 880px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 1120px) {
        .payment-methods-container {
            height: 840px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 1080px) {
        .payment-methods-container {
            height: 800px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 1040px) {
        .payment-methods-container {
            height: 740px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 1000px) {
        .payment-methods-container {
            height: 700px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 960px) {
        .payment-methods-container {
            height: 660px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 920px) {
        .payment-methods-container {
            height: 620px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 880px) {
        .payment-methods-container {
            height: 580px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 840px) {
        .payment-methods-container {
            height: 540px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 800px) {
        .payment-methods-container {
            height: 500px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 760px) {
        .payment-methods-container {
            height: 460px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 720px) {
        .payment-methods-container {
            height: 420px;
            overflow-y: scroll;
        }
    }
</style>
