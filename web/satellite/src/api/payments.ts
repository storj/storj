// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import {
    AccountBalance,
    Coupon,
    CreditCard,
    PaymentsApi,
    PaymentsHistoryItem,
    ProjectUsageAndCharges,
    TokenDeposit
} from '@/types/payments';
import { HttpClient } from '@/utils/httpClient';
import { Time } from '@/utils/time';
import { ErrorTooManyRequests } from './errors/ErrorTooManyRequests';

/**
 * PaymentsHttpApi is a http implementation of Payments API.
 * Exposes all payments-related functionality
 */
export class PaymentsHttpApi implements PaymentsApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/payments';

    /**
     * Get account balance.
     *
     * @returns balance in cents
     * @throws Error
     */
    public async getBalance(): Promise<AccountBalance> {
        const path = `${this.ROOT_PATH}/account/balance`;
        const response = await this.client.get(path);

        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }

            throw new Error('Can not get account balance');
        }

        const balance = await response.json();
        if (balance) {
            return new AccountBalance(balance.freeCredits, balance.coins);
        }

        return new AccountBalance();
    }

    /**
     * Try to set up a payment account.
     *
     * @throws Error
     */
    public async setupAccount(): Promise<string> {
        const path = `${this.ROOT_PATH}/account`;
        const response = await this.client.post(path, null);
        const couponType = await response.json();

        if (response.ok) {
            return couponType;
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not setup account');
    }

    /**
     * projectsUsageAndCharges returns usage and how much money current user will be charged for each project which he owns.
     */
    public async projectsUsageAndCharges(start: Date, end: Date): Promise<ProjectUsageAndCharges[]> {
        const since = Time.toUnixTimestamp(start).toString();
        const before = Time.toUnixTimestamp(end).toString();
        const path = `${this.ROOT_PATH}/account/charges?from=${since}&to=${before}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }

            throw new Error('can not get projects charges');
        }

        const charges = await response.json();
        if (charges) {
            return charges.map(charge =>
                new ProjectUsageAndCharges(
                    new Date(charge.since),
                    new Date(charge.before),
                    charge.egress,
                    charge.storage,
                    charge.segmentCount,
                    charge.projectId,
                    charge.storagePrice,
                    charge.egressPrice,
                    charge.segmentPrice,
                ),
            );
        }

        return [];
    }

    /**
     * Add credit card.
     *
     * @param token - stripe token used to add a credit card as a payment method
     * @throws Error
     */
    public async addCreditCard(token: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.post(path, token);

        if (response.ok) {
            return;
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not add credit card');
    }

    /**
     * Detach credit card from payment account.
     *
     * @param cardId
     * @throws Error
     */
    public async removeCreditCard(cardId: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards/${cardId}`;
        const response = await this.client.delete(path);

        if (response.ok) {
            return;
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not remove credit card');
    }

    /**
     * Get list of user`s credit cards.
     *
     * @returns list of credit cards
     * @throws Error
     */
    public async listCreditCards(): Promise<CreditCard[]> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.get(path);

        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }
            throw new Error('can not list credit cards');
        }

        const creditCards = await response.json();

        if (creditCards) {
            return creditCards.map(card => new CreditCard(card.id, card.expMonth, card.expYear, card.brand, card.last4, card.isDefault));
        }

        return [];
    }

    /**
     * Make credit card default.
     *
     * @param cardId
     * @throws Error
     */
    public async makeCreditCardDefault(cardId: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.patch(path, cardId);

        if (response.ok) {
            return;
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not make credit card default');
    }

    /**
     * Returns a list of invoices, transactions and all others payments history items for payment account.
     *
     * @returns list of payments history items
     * @throws Error
     */
    public async paymentsHistory(): Promise<PaymentsHistoryItem[]> {
        const path = `${this.ROOT_PATH}/billing-history`;
        const response = await this.client.get(path);

        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }
            throw new Error('can not list billing history');
        }

        const paymentsHistoryItems = await response.json();
        if (paymentsHistoryItems) {
            return paymentsHistoryItems.map(item =>
                new PaymentsHistoryItem(
                    item.id,
                    item.description,
                    item.amount,
                    item.received,
                    item.status,
                    item.link,
                    new Date(item.start),
                    new Date(item.end),
                    item.type,
                    item.remaining,
                ),
            );
        }

        return [];
    }

    /**
     * makeTokenDeposit process coin payments.
     *
     * @param amount
     * @throws Error
     */
    public async makeTokenDeposit(amount: number): Promise<TokenDeposit> {
        const path = `${this.ROOT_PATH}/tokens/deposit`;
        const response = await this.client.post(path, JSON.stringify({ amount }));

        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }

            throw new Error('can not process coin payment');
        }

        const result = await response.json();

        return new TokenDeposit(result.amount, result.address, result.link);
    }

    /**
     * applyCouponCode applies a coupon code.
     *
     * @param couponCode
     * @throws Error
     */
    public async applyCouponCode(couponCode: string): Promise<Coupon> {
        const path = `${this.ROOT_PATH}/coupon/apply`;
        const response = await this.client.patch(path, couponCode);
        const errMsg = `Could not apply coupon code "${couponCode}"`;

        if (response.ok) {
            const coupon = await response.json();

            if (!coupon) {
                throw new Error(errMsg);
            }

            return new Coupon(
                coupon.id,
                coupon.promoCode,
                coupon.name,
                coupon.amountOff,
                coupon.percentOff,
                new Date(coupon.addedAt),
                coupon.expiresAt ? new Date(coupon.expiresAt) : null,
                coupon.duration
            );
        }

        switch (response.status) {
        case 429:
            throw new ErrorTooManyRequests('You\'ve exceeded limit of attempts, try again in 5 minutes');
        case 401:
            throw new ErrorUnauthorized(errMsg);
        default:
            throw new Error(errMsg);
        }
    }

    /**
     * getCoupon returns the coupon applied to the user.
     *
     * @throws Error
     */
    public async getCoupon(): Promise<Coupon | null> {
        const path = `${this.ROOT_PATH}/coupon`;
        const response = await this.client.get(path);
        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }

            throw new Error('cannot retrieve coupon');
        }

        const coupon = await response.json();

        if (!coupon) {
            return null;
        }

        return new Coupon(
            coupon.id,
            coupon.promoCode,
            coupon.name,
            coupon.amountOff,
            coupon.percentOff,
            new Date(coupon.addedAt),
            coupon.expiresAt ? new Date(coupon.expiresAt) : null,
            coupon.duration
        );
    }
}
