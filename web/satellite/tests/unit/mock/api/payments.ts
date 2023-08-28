// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccountBalance,
    Coupon,
    CreditCard,
    PaymentsApi,
    ProjectUsagePriceModel,
    TokenDeposit,
    NativePaymentHistoryItem,
    Wallet,
    ProjectCharges, PaymentHistoryParam, PaymentHistoryPage,
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

    projectsUsageAndCharges(): Promise<ProjectCharges> {
        return Promise.resolve(new ProjectCharges());
    }

    projectUsagePriceModel(): Promise<ProjectUsagePriceModel> {
        return Promise.resolve(new ProjectUsagePriceModel('1', '1', '1'));
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

    paymentsHistory(param: PaymentHistoryParam): Promise<PaymentHistoryPage> {
        return Promise.resolve(new PaymentHistoryPage([]));
    }

    nativePaymentsHistory(): Promise<NativePaymentHistoryItem[]> {
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

    getWallet(): Promise<Wallet> {
        return Promise.resolve(new Wallet());
    }

    claimWallet(): Promise<Wallet> {
        return Promise.resolve(new Wallet());
    }

    purchasePricingPackage(_: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    pricingPackageAvailable(): Promise<boolean> {
        throw new Error('Method not implemented');
    }
}
