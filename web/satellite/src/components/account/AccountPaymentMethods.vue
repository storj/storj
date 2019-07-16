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
            isDisabled="true" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import CardComponent from '@/components/project/paymentMethods/CardComponent.vue';

    @Component({
        computed: {
            paymentMethods: function (): PaymentMethod[] {
                return this.$store.state.userPaymentsMethodsModule.userPaymentMethods;
            }
        },
        components: {
            CardComponent
        }
    })
    export default class AccountPaymentMethods extends Vue {}
</script>

<style scoped lang="scss">
    .payment-methods-container {
        margin-top: 83px;
    }
</style>
