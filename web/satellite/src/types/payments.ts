// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { formatPrice } from '@/utils/strings';
import { JSONRepresentable } from '@/types/json';
import { Time } from '@/utils/time';

/**
 * Page parameters for listing payments history.
 */
export interface PaymentHistoryParam {
    limit: number;
    startingAfter: string;
    endingBefore: string;
}

/**
 * Exposes all payments-related functionality
 */
export interface PaymentsApi {
    /**
     * Try to set up a payment account
     *
     * @throws Error
     */
    setupAccount(csrfProtectionToken: string): Promise<string>;

    /**
     * Get account balance
     *
     * @returns account balance object. Represents free credits and coins in cents.
     * @throws Error
     */
    getBalance(): Promise<AccountBalance>;

    /**
     * projectUsagePriceModel returns the project usage price model for the user.
     */
    projectUsagePriceModel(): Promise<UsagePriceModel>;

    /**
     * productsUsageAndCharges returns usage and how much money current user will be charged for each project which he owns split by product.
     */
    productsUsageAndCharges(since: Date, before: Date): Promise<ProductCharges>;

    /**
     * getPlacementPriceModel returns the usage price model for the user and placement.
     */
    getPlacementPriceModel(params: PriceModelForPlacementRequest): Promise<UsagePriceModel>;

    /**
     * Add funds from a credit card.
     * @param cardID - the ID of the credit card to charge
     * @param amount - the amount of funds to add, in cents
     * @param intent - the intent of the charge, either to add funds or upgrade account
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    addFunds(cardID: string, amount: number, intent: ChargeCardIntent, csrfProtectionToken: string): Promise<AddFundsResponse>;

    /**
     * Creates a payment intent to add funds to the account.
     * @param amount - the amount of funds to add, in cents
     * @param withCustomCard - indicates if new intent should support a custom card
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    createIntent(amount: number, withCustomCard: boolean, csrfProtectionToken: string): Promise<string>;

    /**
     * Gets a setup intent secret to set up a card with stripe.
     *
     * @return string - the client secret for the stripe setup intent.
     * @throws Error
     */
    getCardSetupSecret(): Promise<string>;

    /**
     * Update credit card
     * @param params - the parameters to update the card with.
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    updateCreditCard(params: UpdateCardParams, csrfProtectionToken: string): Promise<void>;

    /**
     * Add payment method.
     * @param request - add card request
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    addCardByPaymentMethodID(request: AddCardRequest, csrfProtectionToken: string): Promise<void>;

    /**
     * Attempt to pay overdue invoices.
     */
    attemptPayments(csrfProtectionToken: string): Promise<void>;

    /**
     * Detach credit card from payment account.
     * @param cardId
     * @param csrfProtectionToken
     * @throws Error
     */
    removeCreditCard(cardId: string, csrfProtectionToken: string): Promise<void>;

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
     * @param csrfProtectionToken
     * @throws Error
     */
    makeCreditCardDefault(cardId: string, csrfProtectionToken: string): Promise<void>;

    /**
     * Returns a list of invoices, transactions and all others payments history items for payment account.
     *
     * @returns list of payments history items
     * @throws Error
     */
    paymentsHistory(param: PaymentHistoryParam): Promise<PaymentHistoryPage>;

    /**
     * Returns a list of invoices, transactions and all others payments history items for payment account.
     *
     * @returns list of payments history items
     * @throws Error
     */
    nativePaymentsHistory(): Promise<NativePaymentHistoryItem[]>;

    /**
     * Returns a list of STORJ token payments with confirmations.
     *
     * @returns list of native token payment items with confirmations
     * @throws Error
     */
    paymentsWithConfirmations(): Promise<PaymentWithConfirmations[]>;

    /**
     * applyCouponCode applies a coupon code.
     *
     * @param couponCode
     * @param csrfProtectionToken
     * @throws Error
     */
    applyCouponCode(couponCode: string, csrfProtectionToken: string): Promise<Coupon>;

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
    claimWallet(csrfProtectionToken: string): Promise<Wallet>;

    /**
     * get user's billing information.
     *
     * @throws Error
     */
    getBillingInformation(): Promise<BillingInformation>;

    /**
     * add user's default invoice reference.
     *
     * @param reference - invoice reference to be shown on invoices
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    addInvoiceReference(reference: string, csrfProtectionToken: string): Promise<BillingInformation>;

    /**
     * save user's billing information.
     *
     * @param address - billing information to save
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    saveBillingAddress(address: BillingAddress, csrfProtectionToken: string): Promise<BillingInformation>;

    /**
     * get a list of countries whose taxes are supported.
     *
     * @throws Error
     */
    getTaxCountries(): Promise<TaxCountry[]>;

    /**
     * get a list of supported taxes for a country.
     *
     * @throws Error
     */
    getCountryTaxes(countryCode: string): Promise<Tax[]>;

    /**
     * add a tax ID to a user's account.
     *
     * @param type - tax type
     * @param value - tax value
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    addTaxID(type: string, value: string, csrfProtectionToken: string): Promise<BillingInformation>;

    /**
     * remove a tax ID from a user's account.
     *
     * @param taxID - the tax ID to remove
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    removeTaxID(taxID: string, csrfProtectionToken: string): Promise<BillingInformation>;

    /**
     * Purchases makes a purchase using a credit card action.
     * Used for pricing packages and upgrade account.
     *
     * @param request - purchase request
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    purchase(request: PurchaseRequest, csrfProtectionToken: string): Promise<void>;

    /**
     * Returns whether there is a pricing package configured for the user's partner.
     *
     * @throws Error
     */
    pricingPackageAvailable(): Promise<boolean>;

    /**
     * startFreeTrial starts a free trial for the user.
     * @param csrfProtectionToken
     */
    startFreeTrial(csrfProtectionToken: string): Promise<void>;
}

export class AccountBalance {
    constructor(
        public freeCredits: number = 0,
        // STORJ token balance (in dollars) from storjscan.
        private _coins: string = '0',
        // STORJ balance (in cents) from stripe. This may include the following.
        // 1. legacy Coinpayments deposit.
        // 2. legacy credit for a manual STORJ deposit.
        // 4. bonus manually credited for a storjscan payment once a month before  invoicing.
        // 5. any other adjustment we may have to make from time to time manually to the customer´s STORJ balance.
        private _credits: string = '0',
    ) { }

    public get coins(): number {
        return parseFloat(this._coins);
    }

    public get credits(): number {
        return parseFloat(this._credits);
    }

    // Returns sum of storjscan and legacy (stripe) balances in cents.
    public get sum(): number {
        return this.credits + (this.coins * 100);
    }

    public get formattedSum(): string {
        return formatPrice((this.sum / 100).toLocaleString(undefined, { maximumFractionDigits: 2 }));
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

    public get isExpiring(): boolean {
        const now = new Date();

        return now.getFullYear() === this.expYear && now.getMonth() + 1 === this.expMonth;
    }

    public get isExpired(): boolean {
        const now = new Date();
        const month = now.getMonth();
        const year = now.getFullYear();

        return year > this.expYear || (year === this.expYear && month + 1 > this.expMonth);
    }
}

/**
 * PaymentHistoryPage holds a paged list of PaymentsHistoryItem.
 */
export class PaymentHistoryPage {
    public constructor(
        public readonly items: PaymentsHistoryItem[],
        public readonly hasNext = false,
        public readonly hasPrevious = false,
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
        public readonly status: PaymentsHistoryItemStatus = PaymentsHistoryItemStatus.Pending,
        public readonly link: string = '',
        public readonly start: Date = new Date(),
        public readonly end: Date = new Date(),
        public readonly type: PaymentsHistoryItemType = PaymentsHistoryItemType.Invoice,
        public readonly remaining: number = 0,
    ) { }

    public get formattedStatus(): string {
        return this.status.charAt(0).toUpperCase() + this.status.substring(1);
    }

    public get period(): string {
        return Time.formattedDate(this.start) + ' - ' + Time.formattedDate(this.end);
    }

    public get label(): string {
        switch (this.type) {
        case PaymentsHistoryItemType.Transaction:
            return 'Checkout';
        default:
            return 'Invoice PDF';
        }
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

    /**
     * Status showed if transaction has not finalized yet.
     */
    Draft = 'draft',

    /**
     * This is to filter when the backend sends an item with an empty status.
     */
    Empty = '',
}

export enum ChargeCardIntent {
    AddFunds = 1,
}

export interface AddCardRequest {
    token: string;
    address?: PurchaseAddress;
    tax?: PurchaseTax;
}

export enum PurchaseIntent {
    PackagePlan = 1,
    UpgradeAccount = 2,
}

export interface PurchaseRequest extends AddCardRequest {
    intent: PurchaseIntent;
}

export interface PurchaseAddress {
    name: string;
    line1: string;
    line2?: string | null;
    city: string;
    state: string;
    postalCode: string;
    country: string;
}

export interface PurchaseTax {
    type: string;
    value: string;
}

export interface PurchaseBillingInfo {
    address?: PurchaseAddress;
    tax?: PurchaseTax
}

/**
 * ProjectCharge shows usage and how much money current project will charge in the end of the month.
  */
export class ProjectCharge {
    public constructor(
        public since: Date = new Date(),
        public before: Date = new Date(),
        public egress: number = 0,
        public storage: number = 0,
        public segmentCount: number = 0,
        // storage shows how much cents we should pay for storing GB*Hrs.
        public storagePrice: number = 0,
        // egress shows how many cents we should pay for Egress.
        public egressPrice: number = 0,
        // segmentCount shows how many cents we should pay for segments count.
        public segmentPrice: number = 0,
        public includedEgress: number = 0,
    ) { }

    /**
     * summary returns total price for a project in cents.
     */
    public get summary(): number {
        return this.storagePrice + this.egressPrice + this.segmentPrice;
    }
}

/**
 * ProductCharge shows usage and how much money current project will charge in the end of the month split by product.
 */
export class ProductCharge extends ProjectCharge {
    constructor(
        public productID: string = '',
        public productName: string = '',
        public priceModel: UsagePriceModel = new UsagePriceModel(),
        since: Date = new Date(),
        before: Date = new Date(),
        egress: number = 0,
        storage: number = 0,
        segmentCount: number = 0,
        storagePrice: number = 0,
        egressPrice: number = 0,
        segmentPrice: number = 0,
        includedEgress: number = 0,
    ) {
        super(since, before, egress, storage, segmentCount, storagePrice, egressPrice, segmentPrice, includedEgress);
    }
}

/**
 * The JSON representation of ProductCharges returned from the API.
 */
type ProductChargesJSON = {
    charges: {
        [projectID: string]: {
            [productID: number]: JSONRepresentable<ProductCharge> & {
                since: string;
                before: string;
                egressMBCents: string;
                storageMBMonthCents: string;
                segmentMonthCents: string;
                smallObjectFeeCents: string;
                minimumRetentionFeeCents: string;
                egressOverageMode: boolean;
                egressDiscountRatio: number;
            };
        };
    }
    applyMinimumCharge: boolean;
};

/**
 * Represents a collection of project usage charges grouped by project ID and product ID.
 */
export class ProductCharges {
    private map = new Map<string, Map<number, ProductCharge>>();
    public applyMinimumCharge = false;

    /**
     * Set the usage charge for a project and product.
     *
     * @param projectID - The ID of the project.
     * @param productID - The ID of the product.
     * @param charge - The usage and charges for the project and product.
     */
    public set(projectID: string, productID: number, charge: ProductCharge): void {
        const map = this.map.get(projectID) || new Map<number, ProductCharge>();
        map.set(productID, charge);
        this.map.set(projectID, map);
    }

    /**
     * Returns the usage charge for a project and partner or undefined if it does not exist.
     *
     * @param projectID - The ID of the project.
     * @param productID - The ID of the product.
     */
    public get(projectID: string, productID: number): ProductCharge | undefined {
        const map = this.map.get(projectID);
        if (!map) return undefined;
        return map.get(productID);
    }

    /**
     * Returns the sum of all usage charges.
     */
    public getPrice(): number {
        let sum = 0;
        this.forEachCharge(charge => {
            sum += charge.summary;
        });
        return sum;
    }

    /**
     * Returns the project usage price model for a product or undefined if it does not exist.
     *
     * @param projectID - The ID of the project.
     * @param productID - The ID of the product.
     */
    public getUsagePriceModel(projectID: string, productID: number): UsagePriceModel | undefined {
        const map = this.map.get(projectID);
        if (!map) return undefined;
        return map.get(productID)?.priceModel;
    }

    /**
     * Returns the product name or undefined if it does not exist.
     *
     * @param projectID - The ID of the project.
     * @param productID - The ID of the product.
     */
    public getProductName(projectID: string, productID: number): string | undefined {
        const map = this.map.get(projectID);
        if (!map) return undefined;
        return map.get(productID)?.productName;
    }

    /**
     * Returns the sum of all usage charges for the project ID.
     *
     * @param projectID - The ID of the project.
     */
    public getProjectPrice(projectID: string): number {
        let sum = 0;
        this.forEachProjectCharge(projectID, charge => {
            sum += charge.summary;
        });
        return sum;
    }

    /**
     * Returns whether this collection contains information for a project.
     *
     * @param projectID - The ID of the project.
     */
    public hasProject(projectID: string): boolean {
        return this.map.has(projectID);
    }

    /**
     * Iterate over each usage charge for all projects and products.
     *
     * @param callback - A function to be called for each usage charge.
     */
    public forEachCharge(callback: (charge: ProductCharge, productID: number, projectID: string) => void): void {
        this.map.forEach((productCharges, projectID) => {
            productCharges.forEach((charge, productID) => {
                callback(charge, productID, projectID);
            });
        });
    }

    /**
     * Calls a provided function once for each usage charge associated with a given project.
     *
     * @param projectID The project ID for which to iterate over usage charges.
     * @param callback The function to call for each usage charge, taking the charge object, product ID, and project ID as arguments.
     */
    public forEachProjectCharge(projectID: string, callback: (charge: ProductCharge, productID: number) => void): void {
        const productCharges = this.map.get(projectID);
        if (!productCharges) return;
        productCharges.forEach((charge, productID) => {
            callback(charge, productID);
        });
    }

    /**
     * Returns the collection as an array of nested arrays, where each inner array represents a project and its
     * associated product charges. The inner arrays have the format [projectID, [productCharge1, productCharge2, ...]],
     * where each productCharge is a [productID, charge] tuple.
     */
    public toArray(): [projectID: string, productCharges: [productID: number, charge: ProductCharge][]][] {
        const result: [string, [number, ProductCharge][]][] = [];
        this.map.forEach((productCharges, projectID) => {
            const productChargeArray: [number, ProductCharge][] = [];
            productCharges.forEach((charge, productID) => {
                productChargeArray.push([productID, charge]);
            });
            result.push([projectID, productChargeArray]);
        });
        return result;
    }

    /**
     * Returns a new ProjectProductCharges instance from a JSON representation.
     *
     * @param json - The JSON representation of the ProjectProductCharges.
     */
    public static fromJSON(json: ProductChargesJSON): ProductCharges {
        const charges = new ProductCharges();
        charges.applyMinimumCharge = json.applyMinimumCharge;

        for (const [projectID, productMap] of Object.entries(json.charges)) {
            for (const [productIDKey, chargeJSON] of Object.entries(productMap)) {
                const productIDNum = Number(productIDKey);

                const {
                    productID: pidStr,
                    productName,
                    storageMBMonthCents,
                    egressMBCents,
                    segmentMonthCents,
                    since: sinceStr,
                    before: beforeStr,
                    egress,
                    storage,
                    segmentCount,
                    storagePrice,
                    egressPrice,
                    segmentPrice,
                    egressOverageMode,
                    includedEgress,
                    smallObjectFeeCents,
                    minimumRetentionFeeCents,
                    egressDiscountRatio,
                } = chargeJSON;

                const pc = new ProductCharge(
                    pidStr,
                    productName,
                    new UsagePriceModel(storageMBMonthCents, egressMBCents, segmentMonthCents, smallObjectFeeCents, minimumRetentionFeeCents, egressOverageMode, egressDiscountRatio),
                    new Date(sinceStr),
                    new Date(beforeStr),
                    egress,
                    storage,
                    segmentCount,
                    storagePrice,
                    egressPrice,
                    segmentPrice,
                    includedEgress,
                );

                charges.set(projectID, productIDNum, pc);
            }
        }

        return charges;
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
        public partnered: boolean = false,
    ) { }
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
    Forever = 'forever',
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

    public get formattedAmount(): string {
        if (this.type === 'coinpayments') {
            return this.received.formattedValue;
        }
        return this.amount.formattedValue;
    }
}

export enum PaymentStatus {
    Pending = 'pending',
    Confirmed = 'confirmed',
}

/**
 * PaymentWithConfirmation holds all information about token payment with confirmations count.
 */
export class PaymentWithConfirmations {
    public constructor(
        public readonly address: string = '',
        public readonly tokenValue: number = 0,
        public readonly usdValue: number = 0,
        public readonly transaction: string = '',
        public readonly timestamp: Date = new Date(),
        public readonly bonusTokens: number = 0,
        public status: PaymentStatus = PaymentStatus.Confirmed,
        public confirmations: number = 0,
    ) { }
}

export class TokenAmount {
    public constructor(
        private readonly _value: string = '0.0',
        public readonly currency: string = '',
    ) { }

    public get value(): number {
        return Number.parseFloat(this._value);
    }

    public get formattedValue(): string {
        return formatPrice(this.value.toLocaleString(undefined, { maximumFractionDigits: 2 }));
    }
}

/**
 * ProjectUsagePriceModel represents price model for project usage.
 */
export class UsagePriceModel {
    public constructor(
        public readonly storageMBMonthCents: string = '',
        public readonly egressMBCents: string = '',
        public readonly segmentMonthCents: string = '',
        public readonly smallObjectFeeCents: string = '',
        public readonly minimumRetentionFeeCents: string = '',
        public readonly egressOverageMode: boolean = false,
        public readonly egressDiscountRatio: number = 0,
    ) { }
}

export interface AddFundsResponse {
    success: boolean,
    clientSecret: string,
    paymentIntentID: string,
}

export interface TaxCountry {
    code: string,
    name?: string,
}

export interface Tax {
    code: string,
    name?: string,
    example?: string,
    countryCode?: string,
}

export interface BillingAddress {
    name: string,
    line1: string,
    line2?: string | null,
    city: string,
    state?: string,
    postalCode?: string,
    country: TaxCountry,
}

export interface TaxID {
    id?: string,
    value: string,
    tax: Tax,
}

export interface BillingInformation {
    address?: BillingAddress,
    taxIDs?: TaxID[],
    invoiceReference: string,
}

export interface UpdateCardParams {
    cardID:  string
    expMonth: number
    expYear: number
}

export interface PriceModelForPlacementRequest {
    placementName?: string;
    placement?: number;
    projectID: string;
}
