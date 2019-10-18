// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class CreditCard {
    public isSelected: boolean = false;

    constructor(
        public id: string = '',
        public expMonth: number = 0,
        public expYear: number = 0,
        public brand: string = '',
        public last4: string = '0000',
        public isDefault: boolean = false,
    ) {}
}

/**
 * Exposes all payments-related functionality
 */
export interface PaymentsApi {
    /**
     * Fetch apiKeys
     *
     * @throws Error
     */
    setupAccount(): Promise<void>;

    /**
     * Get account balance
     *
     * @returns balance in cents
     * @throws Error
     */
    getBalance(): Promise<number>;

    /**
     * Add credit card
     *
     * @throws Error
     */
    addCreditCard(token: string): Promise<void>;

    /**
     * Get list of user`s credit cards
     *
     * @returns list of credit cards
     * @throws Error
     */
    listCreditCards(): Promise<CreditCard[]>;

    /**
     * Make credit card default
     *
     * @throws Error
     */
    makeCreditCardDefault(id: string): Promise<void>;
}
