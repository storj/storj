// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccountBalance,
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
    setupAccount(): Promise<void> {
        throw new Error('Method not implemented');
    }

    getBalance(): Promise<AccountBalance> {
        return Promise.resolve(new AccountBalance());
    }

    projectsUsageAndCharges(): Promise<ProjectUsageAndCharges[]> {
        return Promise.resolve([]);
    }

    addCreditCard(token: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    removeCreditCard(cardId: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    listCreditCards(): Promise<CreditCard[]> {
        return Promise.resolve([]);
    }

    makeCreditCardDefault(cardId: string): Promise<void> {
        throw new Error('Method not implemented');
    }

    paymentsHistory(): Promise<PaymentsHistoryItem[]> {
        return Promise.resolve([]);
    }

    makeTokenDeposit(amount: number): Promise<TokenDeposit> {
        return Promise.resolve(new TokenDeposit(amount, 'testAddress', 'testLink'));
    }

    getPaywallStatus(userId: string): Promise<boolean> {
        throw new Error('Method not implemented');
    }
}
