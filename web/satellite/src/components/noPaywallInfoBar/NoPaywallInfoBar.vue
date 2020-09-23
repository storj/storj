// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./noPaywallInfoBar.html"/>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';

@Component
export default class NoPaywallInfoBar extends Vue {
    private readonly ONE_QUARTER = 25; // in percents.
    public readonly billingPath: string = RouteConfig.Account.with(RouteConfig.Billing).path;

    /**
     * Indicates if default message is shown.
     */
    public get isDefaultMessage(): boolean {
        return this.promotionalCoupon.remainingAmountPercentage() > this.ONE_QUARTER;
    }

    /**
     * Indicates if warning message is shown.
     */
    public get isWarningMessage(): boolean {
        const remaining = this.promotionalCoupon.remainingAmountPercentage();

        return remaining > 0 && remaining <= this.ONE_QUARTER;
    }

    /**
     * Indicates if error message is shown.
     */
    public get isErrorMessage(): boolean {
        return this.promotionalCoupon.remainingAmountPercentage() === 0;
    }

    /**
     * Returns promotional coupon.
     */
    private get promotionalCoupon(): PaymentsHistoryItem {
        const coupons: PaymentsHistoryItem[] = this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Coupon;
        });

        return coupons[coupons.length - 1] || new PaymentsHistoryItem(); // returns new item in case when coupons array is empty.
    }
}
</script>

<style scoped lang="scss" src="./noPaywallInfoBar.scss"/>
