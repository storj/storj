// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container">
        <div v-for="method in paymentMethods" class="payment-methods-container__card-container">
            <CardComponent :editable="false" :paymentMethod="method"/>
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
</style>
