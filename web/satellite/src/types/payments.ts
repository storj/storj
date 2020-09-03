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
     * @returns account balance object. Represents free credits and coins in cents.
     * @throws Error
     */
    getBalance(): Promise<AccountBalance>;

    /**
     * projectsUsagesAndCharges returns usage and how much money current user will be charged for each project which he owns.
     */
    projectsUsageAndCharges(since: Date, before: Date): Promise<ProjectUsageAndCharges[]>;

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
     * Returns a list of invoices, transactions and all others payments history items for payment account.
     *
     * @returns list of payments history items
     * @throws Error
     */
    paymentsHistory(): Promise<PaymentsHistoryItem[]>;

    /**
     * Creates token transaction in CoinPayments
     *
     * @param amount
     * @throws Error
     */
    makeTokenDeposit(amount: number): Promise<TokenDeposit>;

    /**
     * Indicates if paywall is enabled.
     *
     * @param userId
     * @throws Error
    */
    getPaywallStatus(userId: string): Promise<boolean>;
}

export class AccountBalance {
    constructor(
        public freeCredits: number = 0,
        public coins: number = 0,
    ) {}

    public get sum(): number {
        return this.freeCredits + this.coins;
    }
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

/**
 * PaymentsHistoryItem holds all public information about payments history line.
 */
export class PaymentsHistoryItem {
    public constructor(
        public readonly id: string = '',
        public readonly description: string = '',
        public readonly amount: number = 0,
        public readonly received: number = 0,
        public readonly status: string = '',
        public readonly link: string = '',
        public readonly start: Date = new Date(),
        public readonly end: Date = new Date(),
        public readonly type: PaymentsHistoryItemType = PaymentsHistoryItemType.Invoice,
        public readonly remaining: number = 0,
    ) {}

    public get quantity(): Amount {
        if (this.type === PaymentsHistoryItemType.Transaction) {
            return new Amount('USD $', this.amountDollars(this.amount), this.amountDollars(this.received));
        }

        return new Amount('USD $', this.amountDollars(this.amount));
    }

    public get formattedStatus(): string {
        return this.status.charAt(0).toUpperCase() + this.status.substring(1);
    }

    /**
     * RemainingAmountPercentage will return remaining amount of item in percentage.
     */
    public remainingAmountPercentage(): number {
        if (this.amount === 0) {
            return 0;
        }

        return this.remaining / this.amount * 100;
    }

    private amountDollars(amount): number {
        return amount / 100;
    }

    public downloadLinkHtml(): string {
        if (!this.link) {
            return '';
        }

        const downloadLabel = this.type === PaymentsHistoryItemType.Transaction ? 'Checkout' : 'Invoice PDF';

        return `<a class="download-link" target="_blank" href="${this.link}">${downloadLabel}</a>`;
    }

    /**
     * isTransactionOrDeposit indicates if payments history item type is transaction or deposit bonus.
     */
    public isTransactionOrDeposit(): boolean {
        return this.type === PaymentsHistoryItemType.Transaction || this.type === PaymentsHistoryItemType.DepositBonus;
    }
}

/**
 * PaymentsHistoryItemType indicates type of history item.
  */
export enum PaymentsHistoryItemType {
    // Invoice is a Stripe invoice billing item.
    Invoice = 0,
    // Transaction is a Coinpayments transaction billing item.
    Transaction = 1,
    // Charge is a credit card charge billing item.
    Charge = 2,
    // Coupon is a promotional coupon item.
    Coupon = 3,
    // DepositBonus is a 10% bonus for using Coinpayments transactions.
    DepositBonus = 4,
}

/**
 * PaymentsHistoryItemStatus indicates status of history item.
 */
export enum PaymentsHistoryItemStatus {
    /**
     * Status showed if transaction successfully completed.
     */
    Completed = 'completed',

    /**
     * Status showed if transaction successfully paid.
     */
    Paid = 'paid',

    /**
     * Status showed if transaction is pending.
     */
    Pending = 'pending',
}

/**
 * TokenDeposit holds public information about token deposit.
 */
export class TokenDeposit {
    constructor(
        public amount: number,
        public address: string,
        public link: string,
    ) {}
}

/**
 * Amount holds information for displaying billing item payment.
 */
class Amount {
    public constructor(
        public currency: string = '',
        public total: number = 0,
        public received: number = 0,
    ) {}
}

/**
 * ProjectUsageAndCharges shows usage and how much money current project will charge in the end of the month.
  */
export class ProjectUsageAndCharges {
    public constructor(
        public since: Date = new Date(),
        public before: Date = new Date(),
        public egress: number = 0,
        public storage: number = 0,
        public objectCount: number = 0,
        public projectId: string = '',
        // storage shows how much cents we should pay for storing GB*Hrs.
        public storagePrice: number = 0,
        // egress shows how many cents we should pay for Egress.
        public egressPrice: number = 0,
        // objectCount shows how many cents we should pay for objects count.
        public objectPrice: number = 0) {}

    /**
     * summary returns total price for a project in cents.
     */
    public summary(): number {
        return this.storagePrice + this.egressPrice + this.objectPrice;
    }
}

/**
 * Holds start and end dates.
 */
export class DateRange {
    public startDate: Date = new Date();
    public endDate: Date = new Date();

    public constructor(startDate: Date, endDate: Date) {
        this.startDate = startDate;
        this.endDate = endDate;
    }
}
