// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="chosen-card-container">
            <Card
                isChosen
                :lastDigits="defaultCard.lastFour"
                expireLabel="Expires:"
                :expireDate="`${defaultCard.expMonth} / ${defaultCard.expYear}`"/>
            <div class="chosen-card-container__button-area">
                <Button
                    label="Default"
                    width="91px"
                    height="36px"
                    :onPress="stub"
                    isDisabled="true"/>
                <div @click="onToggleSelectionModeClick" class="chosen-card-container__expand-container"
                     v-if="isPaymentSelectorEnabled">
                    <h3>{{dropdownTitle}} </h3>
                    <img src="@/../static/images/payments/circle.svg"/>
                </div>
            </div>
        </div>
        <div class="border"></div>
        <div class="expanded-area" :class="{'empty': !isPaymentSelectorShown}" >
            <div v-if="isPaymentSelectorShown" >
                <div v-for="method in userPaymentMethods">
                    <UserPaymentMethodCardComponent :method="method" :key="method.id"/>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import Card from '@/components/project/CardChoiceItem.vue';
    import UserPaymentMethodCardComponent from '@/components/project/paymentMethods/UserPaymentMethodCardComponent.vue';
    import { PaymentMethod } from '@/types/invoices';

    @Component({
        components: {
            Button,
            Card,
            UserPaymentMethodCardComponent
        }
    })

    export default class PaymentMethodsSelector extends Vue {
        private isPaymentSelectorEnabled = this.userPaymentMethods.length > 1;

        private isPaymentSelectorShown = false;

        public get dropdownTitle(): string {
            return this.isPaymentSelectorShown ? 'Hide' : 'Choose';
        }

        private get userPaymentMethods(): PaymentMethod[] {
            return this.$store.state.userPaymentsMethodsModule.userPaymentMethods;
        }

        private onToggleSelectionModeClick(): void {
            this.isPaymentSelectorShown = !this.isPaymentSelectorShown;
        }

        private get defaultCard(): PaymentMethod {
            return this.$store.state.userPaymentsMethodsModule.defaultPaymentMethod;
        }

        private stub(): void {
            return;
        }
    }
</script>

<style scoped lang="scss">
    h3 {
        width: 72px;
        font-family: 'font_regular';
        font-size: 16px;
        line-height: 23px;
        color: #2683ff;
        text-align: right;
        margin-right: 15px;
    }

    .chosen-card-container {
        display: flex;
        justify-content: space-between;
        align-items: center;
        width: 100%;
        margin-top: 60px;

        &__button-area {
            display: flex;
            align-items: center;
        }

        &__expand-container {
            display: flex;
            margin-left: 10px;
        }

    }

    .border {
        margin-top: 20px;
        width: 100%;
        height: 1px;
        background-color: rgba(169, 181, 193, 0.5);
    }

    .empty {
        background-color: #ffffff !important;
    }

    .expanded-area {
        width: 100%;
        height: 200px;
        overflow-y: auto;
        display: flex;
        background-color: #F5F6FA;
        flex-direction: column;

        &__item {
            display: flex;
            padding-right: 28px;
            align-items: center;
            justify-content: space-between;
            margin-top: 20px;
        }

    }

    .option {
        width: 100%;
    }

</style>
