// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { CreditCard, PaymentsApi } from '@/types/payments';
import { HttpClient } from '@/utils/httpClient';

/**
 * PaymentsHttpApi is a http implementation of Payments API.
 * Exposes all payments-related functionality
 */
export class PaymentsHttpApi implements PaymentsApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/payments';

    /**
     * Get account balance
     *
     * @returns balance in cents
     * @throws Error
     */
    public async getBalance(): Promise<number> {
        const path = `${this.ROOT_PATH}/account/balance`;
        const response = await this.client.get(path);

        if (response.ok) {
            return await response.json();
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not get balance');
    }

    /**
     * Try to set up a payment account
     *
     * @throws Error
     */
    public async setupAccount(): Promise<void> {
        const path = `${this.ROOT_PATH}/account`;
        const response = await this.client.post(path, null);

        if (response.ok) {
            return;
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not setup account');
    }

    /**
     * Add credit card
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
     * Get list of user`s credit cards
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
     * Make credit card default
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
}
