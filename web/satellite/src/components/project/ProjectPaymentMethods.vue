// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container">
        <p class="payment-methods-container__title">Credit or Debit Cards</p>
        <div class="scroll-container">
            <NewPaymentMethodPopup/>
            <div v-for="method in paymentMethods" class="payment-methods-container__card-container">
                <CardComponent :paymentMethod="method"/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import CardComponent from '@/components/project/paymentMethods/CardComponent.vue';
    import NewPaymentMethodPopup from '@/components/project/paymentMethods/NewPaymentMethodComponent.vue';

    @Component({
        components: {
            NewPaymentMethodPopup,
            CardComponent,
        }
    })
    export default class ProjectPaymentMethods extends Vue {
        public get paymentMethods(): PaymentMethod[] {
            return this.$store.state.projectPaymentsMethodsModule.paymentMethods;
        }
    }
</script>

<style scoped lang="scss">
    .payment-methods-container {
        position: relative;

        &__title {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #384B65;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }

         .scroll-container {
             height: 720px;
             overflow-y: scroll;
             overflow-x: hidden;
         }

    }
    ::-webkit-scrollbar {
        width: 0;
    }

</style>
