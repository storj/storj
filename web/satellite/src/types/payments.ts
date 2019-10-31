// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all payments-related functionality
 */
export interface PaymentsApi {
    /**
     * Try to set up a payment account
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
     * @param token - stripe token used to add a credit card as a payment method
     * @throws Error
     */
    addCreditCard(token: string): Promise<void>;

    /**
     * Detach credit card from payment account.
     * @param cardId
     * @throws Error
     */
    removeCreditCard(cardId: string): Promise<void>;

    /**
     * Get list of user`s credit cards
     *
     * @returns list of credit cards
     * @throws Error
     */
    listCreditCards(): Promise<CreditCard[]>;

    /**
     * Make credit card default
     * @param cardId
     * @throws Error
     */
    makeCreditCardDefault(cardId: string): Promise<void>;
}

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

export class PaymentAmountOption {
    public constructor(
        public value: number,
        public label: string = '',
    ) {}
}
