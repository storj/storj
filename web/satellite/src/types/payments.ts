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
    setupAccount(): Promise<string>;

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
     * Returns a list of invoices, transactions and all others payments history items for payment account.
     *
     * @returns list of payments history items
     * @throws Error
     */
    nativePaymentsHistory(): Promise<NativePaymentHistoryItem[]>;

    /**
     * Creates token transaction in CoinPayments
     *
     * @param amount
     * @throws Error
     */
    makeTokenDeposit(amount: number): Promise<TokenDeposit>;

    /**
     * applyCouponCode applies a coupon code.
     *
     * @param couponCode
     * @throws Error
     */
    applyCouponCode(couponCode: string): Promise<Coupon>;

    /**
     * getCoupon returns the coupon applied to the user.
     *
     * @throws Error
     */
    getCoupon(): Promise<Coupon | null>;

    /**
     * get native storj token wallet.
     *
     * @returns wallet
     * @throws Error
     */
    getWallet(): Promise<Wallet>;
    /**
     * claim new native storj token wallet.
     *
     * @returns wallet
     * @throws Error
     */
    claimWallet(): Promise<Wallet>;
}

export class AccountBalance {
    constructor(
        public freeCredits: number = 0,
        public coins: number = 0,
    ) { }

    public get sum(): number {
        return this.freeCredits + this.coins;
    }
}

export class CreditCard {
    public isSelected = false;

    constructor(
        public id: string = '',
        public expMonth: number = 0,
        public expYear: number = 0,
        public brand: string = '',
        public last4: string = '0000',
        public isDefault: boolean = false,
    ) { }
}

export class PaymentAmountOption {
    public constructor(
        public value: number,
        public label: string = '',
    ) { }
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
    ) { }

    public get quantity(): Amount {
        if (this.type === PaymentsHistoryItemType.Transaction) {
            return new Amount('USD $', this.amountDollars(this.amount), this.amountDollars(this.received));
        }

        return new Amount('USD $', this.amountDollars(this.amount));
    }

    public get formattedStatus(): string {
        return this.status.charAt(0).toUpperCase() + this.status.substring(1);
    }

    public get formattedStart(): string {
        return this.start.toLocaleDateString('en-US', { month: 'short', year: 'numeric' });
    }

    public get hasExpiration(): boolean {
        // Go's zero date is passed in if the coupon does not expire
        // Go's zero date is 0001-01-01 00:00:00 +0000 UTC
        // Javascript's zero date is 1970-01-01 00:00:00 +0000 UTC
        return this.end.valueOf() > 0;
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

    public get label(): string {
        switch (this.type) {
        case PaymentsHistoryItemType.Transaction:
            return 'Checkout';
        default:
            return 'Invoice PDF';
        }
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
    ) { }
}

/**
 * Amount holds information for displaying billing item payment.
 */
class Amount {
    public constructor(
        public currency: string = '',
        public total: number = 0,
        public received: number = 0,
    ) { }
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
        public segmentCount: number = 0,
        public projectId: string = '',
        // storage shows how much cents we should pay for storing GB*Hrs.
        public storagePrice: number = 0,
        // egress shows how many cents we should pay for Egress.
        public egressPrice: number = 0,
        // segmentCount shows how many cents we should pay for segments count.
        public segmentPrice: number = 0) { }

    /**
     * summary returns total price for a project in cents.
     */
    public summary(): number {
        return this.storagePrice + this.egressPrice + this.segmentPrice;
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

/**
 * Coupon describes a discount to the payment account of a user.
 */
export class Coupon {
    public constructor(
        public id: string = '',
        public promoCode: string = '',
        public name: string = '',
        public amountOff: number = 0,
        public percentOff: number = 0,
        public addedAt: Date = new Date(),
        public expiresAt: Date | null = new Date(),
        public duration: CouponDuration = CouponDuration.Once,
    ) { }

    /**
     * getDescription returns the amount and duration of the coupon.
     */
    public getDescription(): string {
        let amtOff = `${parseFloat(this.percentOff.toFixed(2)).toString()}%`;
        if (this.amountOff !== 0 || this.percentOff === 0) {
            amtOff = `$${(this.amountOff / 100).toFixed(2).replace('.00', '')}`;
        }
        let dur = this.duration.toString();
        if (this.duration === CouponDuration.Repeating && this.expiresAt !== null) {
            const months = this.expiresAt.getUTCMonth() - this.addedAt.getUTCMonth() +
                (this.expiresAt.getUTCFullYear() - this.addedAt.getFullYear()) * 12;
            dur = `for ${months} month${months !== 1 ? 's' : ''}`;
        }

        return `${amtOff} off ${dur}`;
    }
}

/**
 * CouponDuration indicates how many billing periods a coupon is applied.
 */
export enum CouponDuration {
    /**
     * Indicates that a coupon can only be applied once.
     */
    Once = 'once',

    /**
     * Indicates that a coupon is applied every billing period for a definite amount of time.
     */
    Repeating = 'repeating',

    /**
     * Indicates that a coupon is applied every billing period forever.
     */
    Forever = 'forever'
}

/**
 * Represents STORJ native token payments wallet.
 */
export class Wallet {
    public constructor(
      public address: string = '',
      public balance: TokenAmount = new TokenAmount(),
    ) { }
}

/**
 * TokenPaymentHistoryItem holds all public information about token payments history line.
 */
export class NativePaymentHistoryItem {
    public constructor(
        public readonly id: string = '',
        public readonly wallet: string = '',
        public readonly type: string = '',
        public readonly amount: TokenAmount = new TokenAmount(),
        public readonly received: TokenAmount = new TokenAmount(),
        public readonly status: string = '',
        public readonly link: string = '',
        public readonly timestamp: Date = new Date(),
    ) { }

    public get formattedStatus(): string {
        return this.status.charAt(0).toUpperCase() + this.status.substring(1);
    }

    public get formattedType(): string {
        return this.type.charAt(0).toUpperCase() + this.type.substring(1);
    }
}

export class TokenAmount {
    public constructor(
        private readonly _value: string = '0.0',
        public readonly currency: string = '',
    ) { }

    public get value(): number {
        return Number.parseFloat(this._value);
    }
}
