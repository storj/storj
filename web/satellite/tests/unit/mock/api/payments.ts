// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccountBalance,
    Coupon,
    CreditCard,
    PaymentsApi,
    PaymentsHistoryItem,
    ProjectUsageAndCharges,
    TokenDeposit,
} from '@/types/payments';

/**
 * Mock for PaymentsApi
 */
export class PaymentsMock implements PaymentsApi {
    private mockCoupon: Coupon | null = null;

    public setMockCoupon(coupon: Coupon | null): void {
        this.mockCoupon = coupon;
    }

    setupAccount(): Promise<string> {
        throw new Error('Method not implemented');
    }

    getBalance(): Promise<AccountBalance> {
        return Promise.resolve(new AccountBalance());
    }

    projectsUsageAndCharges(): Promise<ProjectUsageAndCharges[]> {
        return Promise.resolve([]);
    }

    addCreditCard(_token: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    removeCreditCard(_cardId: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    listCreditCards(): Promise<CreditCard[]> {
        return Promise.resolve([]);
    }

    makeCreditCardDefault(_cardId: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    paymentsHistory(): Promise<PaymentsHistoryItem[]> {
        return Promise.resolve([]);
    }

    makeTokenDeposit(amount: number): Promise<TokenDeposit> {
        return Promise.resolve(new TokenDeposit(amount, 'testAddress', 'testLink'));
    }

    applyCouponCode(_: string): Promise<Coupon> {
        throw new Error('Method not implemented');
    }

    getCoupon(): Promise<Coupon | null> {
        return Promise.resolve(this.mockCoupon);
    }
}
