// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-billing-area">
        <AccountBalance />
        <MonthlyBillingSummary />
        <PaymentMethods />
        <DepositAndBilling />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import MonthlyBillingSummary from '@/components/account/billing/MonthlyBillingSummary.vue';
import AccountBalance from '@/components/account/billing/AccountBalance.vue';
import DepositAndBilling from '@/components/account/billing/DepositAndBilling.vue';
import PaymentMethods from '@/components/account/billing/PaymentMethods.vue';

@Component({
    components: {
        AccountBalance,
        MonthlyBillingSummary,
        DepositAndBilling,
        PaymentMethods,
    }
})
export default class BillingArea extends Vue {}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 0;
    }

    .account-billing-area {
        margin-top: 83px;
        overflow-y: scroll;
        height: 75vh;
    }
</style>
