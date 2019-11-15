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

    /**
     * Returns a list of invoices, transactions and all others billing history items for payment account.
     *
     * @returns list of billing history items
     * @throws Error
     */
    billingHistory(): Promise<BillingHistoryItem[]>;

    /**
     *
     * @param amount
     * @throws Error
     */
    makeTokenDeposit(amount: string): Promise<DepositInfo>;
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
        public value: string,
        public label: string = '',
    ) {}
}

// BillingHistoryItem holds all public information about billing history line.
export class BillingHistoryItem {
    public constructor(
        public readonly id: string = '',
        public readonly description: string = '',
        public readonly amount: number = 0,
        public readonly tokenAmount: string = '0',
        public readonly tokenReceived: string = '0',
        public readonly status: string = '',
        public readonly link: string = '',
        public readonly start: Date = new Date(),
        public readonly end: Date = new Date(),
        public readonly type: BillingHistoryItemType = 0,
    ) {}

    public get quantity(): Amount {
        if (this.tokenAmount === '') {
            return new Amount('$', this.amountDollars());
        }

        return new Amount('$', this.tokenAmount, this.tokenReceived);
    }

    private amountDollars(): string {
        return `${this.amount / 100}`;
    }

    public downloadLinkHtml(): string {
        const downloadLabel = this.type === 1 ? 'EtherScan' : 'PDF';

        return `<a class="download-link" href="${this.link}">${downloadLabel}</a>`;
    }
}

// BillingHistoryItemType indicates type of billing history item.
export enum BillingHistoryItemType {
    // Invoice is a Stripe invoice billing item.
    Invoice = 0,
    // Transaction is a Coinpayments transaction billing item.
    Transaction = 1,
}

export class DepositInfo {
    constructor(public amount: string, public address: string) {}
}

class Amount {
    public constructor(
        public currency: string = '',
        public total: string = '0',
        public received: string = '',
    ) {}
}
