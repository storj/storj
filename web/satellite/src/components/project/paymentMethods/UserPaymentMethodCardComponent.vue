// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-method-item" >
        <Card
            class="option"
            :lastDigits="method.lastFour"
            expireLabel="Expires:"
            :expireDate="method.expMonth + '/' + method.expYear"/>

        <Button
            label="Choose"
            width="91px"
            height="36px"
            :isDisabled="isChosen"
            :onPress="()=>setSelectedCard(method)"/>

    </div>

</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import Card from '@/components/project/CardChoiceItem.vue';
    import { USER_PAYMENT_METHODS_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            Button,
            Card,
        }
    })

    export default class UserPaymentMethodCardComponent extends Vue {
        @Prop()
        private readonly method: PaymentMethod;

        public get isChosen(): boolean {
            return this.method.id === this.$store.state.userPaymentsMethodsModule.defaultPaymentMethod.id;
        }

        public setSelectedCard(method: PaymentMethod): void {
            this.$store.dispatch(USER_PAYMENT_METHODS_ACTIONS.SET_DEFAULT, method);
        }
    }
</script>

<style scoped lang="scss">
    .payment-method-item {
        display: flex;
        padding-right: 28px;
        align-items: center;
        justify-content: space-between;
        margin-top: 20px;
    }
</style>
