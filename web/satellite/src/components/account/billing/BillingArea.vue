// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-billing-area">
        <h1 class="account-billing-area__title">Billing</h1>
        <div class="account-billing-area__notification-container">
            <div class="account-billing-area__notification-container__negative-balance" v-if="isBalanceNegative">
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="40" height="40" rx="10" fill="#EB5757"/>
                    <path d="M20.5 22.75C21.7676 22.75 22.8047 21.645 22.8047 20.2944V10.4556C22.8047 10.3328 22.797 10.2019 22.7816 10.0791C22.6126 8.90857 21.6523 8 20.5 8C19.2324 8 18.1953 9.10502 18.1953 10.4556V20.2862C18.1953 21.645 19.2324 22.75 20.5 22.75Z" fill="#F5F5F9"/>
                    <path d="M20.5 25.1465C18.7146 25.1465 17.2734 26.5877 17.2734 28.373C17.2734 30.1584 18.7146 31.5996 20.5 31.5996C22.2853 31.5996 23.7265 30.1584 23.7265 28.373C23.7337 26.5877 22.2925 25.1465 20.5 25.1465Z" fill="#F5F5F9"/>
                </svg>
                <p class="account-billing-area__notification-container__negative-balance__text">Your usage charges exceed your account balance. Please add STORJ Tokens or a debit/credit card to prevent data loss.</p>
            </div>
            <div class="account-billing-area__notification-container__low-balance" v-if="isBalanceLow">
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M37.0275 30.9607C36.5353 30.0514 36.04 29.1404 35.5463 28.2264C34.6275 26.531 33.7103 24.8357 32.7931 23.1404C31.6916 21.1091 30.5931 19.0748 29.4931 17.0436C28.4307 15.0826 27.3713 13.1248 26.3088 11.1656C25.5275 9.72492 24.7494 8.28276 23.9681 6.84076C23.7572 6.45014 23.5463 6.05952 23.3353 5.672C23.1166 5.26576 22.8853 4.87512 22.5541 4.54388C21.3979 3.3798 19.4791 3.15636 18.0885 4.03608C17.4916 4.41421 17.0604 4.95016 16.7291 5.5642C16.2213 6.50172 15.7135 7.4392 15.2057 8.37984C14.2807 10.0908 13.3541 11.8017 12.4291 13.5126C11.3151 15.5548 10.2104 17.6018 9.10102 19.6486C8.05102 21.5893 6.99946 23.5268 5.9479 25.469C5.17914 26.8909 4.40882 28.3097 3.63854 29.7314C3.43541 30.1064 3.2323 30.4814 3.02918 30.8564C2.7448 31.3846 2.52138 31.9189 2.45106 32.5283C2.25262 34.2502 3.45886 35.9471 5.12606 36.369C5.5667 36.4815 6.00106 36.4861 6.44638 36.4861H33.9464H33.9901C34.8964 36.4674 35.7558 36.1268 36.4198 35.5096C37.0604 34.9158 37.4354 34.1033 37.5417 33.2439C37.6432 32.4346 37.4089 31.6689 37.0276 30.9611L37.0275 30.9607ZM18.4367 13.9527C18.4367 13.0777 19.1523 12.4293 19.9992 12.3902C20.8429 12.3512 21.5617 13.1371 21.5617 13.9527V24.9559C21.5617 25.8309 20.846 26.4794 19.9992 26.5184C19.1554 26.5575 18.4367 25.7715 18.4367 24.9559V13.9527ZM19.9992 31.8403C19.1211 31.8403 18.4085 31.1294 18.4085 30.2497C18.4085 29.3716 19.1195 28.659 19.9992 28.659C20.8773 28.659 21.5898 29.37 21.5898 30.2497C21.5898 31.1278 20.8773 31.8403 19.9992 31.8403Z" fill="#F4D638"/>
                </svg>
                <p class="account-billing-area__notification-container__low-balance__text">Your account balance is running low. Please add STORJ Tokens or a debit/credit card to prevent data loss.</p>
            </div>
        </div>
        <AccountBalance/>
        <MonthlyBillingSummary/>
        <PaymentMethods />
        <DepositAndBilling/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AccountBalance from '@/components/account/billing/AccountBalance.vue';
import DepositAndBilling from '@/components/account/billing/DepositAndBilling.vue';
import MonthlyBillingSummary from '@/components/account/billing/MonthlyBillingSummary.vue';
import PaymentMethods from '@/components/account/billing/PaymentMethods.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

const {
    CLEAR_PAYMENT_INFO,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        AccountBalance,
        MonthlyBillingSummary,
        DepositAndBilling,
        PaymentMethods,
    },
})
export default class BillingArea extends Vue {
    // CRITICAL_AMOUNT hold minimum safe balance in cents.
    // If balance is lower - yellow notification should appear.
    private readonly CRITICAL_AMOUNT: number = 1000;

    public beforeDestroy() {
        this.$store.dispatch(CLEAR_PAYMENT_INFO);
    }

    public get isBalanceNegative(): boolean {
        return this.$store.state.paymentsModule.balance < 0;
    }

    public get isBalanceLow(): boolean {
        return this.$store.state.paymentsModule.balance > 0 && this.$store.state.paymentsModule.balance < this.CRITICAL_AMOUNT;
    }
}
</script>

<style scoped lang="scss">
    .account-billing-area {

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            margin: 40px 0 25px 0;
        }

        &__notification-container {

            &__negative-balance,
            &__low-balance {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 20px 20px 20px 20px;
                border-radius: 12px;
                margin-bottom: 32px;

                &__text {
                    font-family: 'font_medium', sans-serif;
                    margin: 0 17px;
                    font-size: 14px;
                    font-weight: 500;
                    line-height: 19px;
                }
            }

            &__negative-balance {
                background-color: #ffd4d2;
            }

            &__low-balance {
                background-color: #fcf8e3;
            }
        }
    }
</style>
