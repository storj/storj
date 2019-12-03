// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BillingHistoryItem, CreditCard, PaymentsApi, ProjectCharge, TokenDeposit } from '@/types/payments';

/**
 * Mock for PaymentsApi
 */
export class PaymentsMock implements PaymentsApi {
    private tokenDeposit: TokenDeposit;

    setupAccount(): Promise<void> {
        throw new Error('Method not implemented');
    }

    getBalance(): Promise<number> {
        return Promise.resolve(0);
    }

    projectsCharges(): Promise<ProjectCharge[]> {
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

    billingHistory(): Promise<BillingHistoryItem[]> {
        return Promise.resolve([]);
    }

    makeTokenDeposit(amount: number): Promise<TokenDeposit> {
        return Promise.resolve(this.tokenDeposit);
    }
}
