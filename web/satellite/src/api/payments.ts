// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

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
     * getBalance exposes http request to grt balance in cents
     */
    public async getBalance(): Promise<number> {
        const path = `${this.ROOT_PATH}/accounts/balance`;
        const response = await this.client.get(path);

        return await response.json();
    }

    public async setupAccount(): Promise<void> {
        const path = `${this.ROOT_PATH}/accounts`;
        const body = {};
        const response = await this.client.post(path, JSON.stringify(body));

        return await response.json();
    }

    public async addCreditCard(): Promise<void> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.get(path);

        return await response.json();
    }

    public async listCreditCards(): Promise<CreditCard[]> {
        const path = `${this.ROOT_PATH}/cards`;
        const body = {};
        const response = await this.client.post(path, JSON.stringify(body));

        return await response.json();
    }
}
