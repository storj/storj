// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="chosen-card-container">
            <Card
                    isChosen
                    lastDigits="0000"
                    fullName="Shawn Wilkinson"
                    expireLabel="Storj Labs"
                    expireDate="12/2020" />
            <div class="chosen-card-container__button-area">
                <Button
                        label="Default"
                        width="91px"
                        height="36px"
                        isDisabled />
                <div class="chosen-card-container__expand-container" v-if="userPaymentMethods.length > 0">
                    <h3>{{dropdownTitle}}</h3>
                    <!--<img :src="showHideImageSource"/>-->
                    <img src="../../../../static/images/payments/circle.svg"/>
                </div>
            </div>
        </div>
        <div class="border"></div>
        <div class="expanded-area">
            <div class="expanded-area__item" v-for="method in userPaymentMethods">
                <Card
                        class="option"
                        :lastDigits="method.lastFour"
                        fullName=holderName
                        expireLabel="Expires:"
                        :expireDate="method.expMonth + '/' +method.expYear" />

                <Button
                        label="Choose"
                        width="91px"
                        height="36px"/>

            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import Card from '@/components/project/CardChoiceItem.vue'

    @Component({
        data: function () {
            return {
            };
        },
        methods: {
            onDoneClick: function (): void {

            },
            onCloseClick: function (): void {

            },
            onNewCardClick: function (): void {

            }
        },
        computed: {
            userPaymentMethods: function (): PaymentMethod[] {
                return this.$store.state.userPaymentsMethodsModule.userPaymentMethods;
            }
        },
        components: {
            Button,
            Card,
        }
    })

    export default class PaymentMethodsSelector extends Vue {
        private dropdownTitle: string = 'hide'
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
