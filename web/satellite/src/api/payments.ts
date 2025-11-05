// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccountBalance,
    Coupon,
    CreditCard,
    PaymentsApi,
    PaymentsHistoryItem,
    UsagePriceModel,
    TokenAmount,
    NativePaymentHistoryItem,
    Wallet,
    PaymentWithConfirmations,
    PaymentHistoryParam,
    PaymentHistoryPage,
    BillingInformation,
    BillingAddress,
    TaxCountry,
    UpdateCardParams,
    PriceModelForPlacementRequest,
    AddFundsResponse,
    ProductCharges,
    ChargeCardIntent,
    PurchaseRequest,
    AddCardRequest,
} from '@/types/payments';
import { HttpClient } from '@/utils/httpClient';
import { Time } from '@/utils/time';
import { APIError } from '@/utils/error';

/**
 * PaymentsHttpApi is a http implementation of Payments API.
 * Exposes all payments-related functionality
 */
export class PaymentsHttpApi implements PaymentsApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/payments';

    /**
     * Starts add funds flow.
     *
     * @throws Error
     */
    public async addFunds(cardID: string, amount: number, intent: ChargeCardIntent, csrfProtectionToken: string): Promise<AddFundsResponse> {
        const path = `${this.ROOT_PATH}/add-funds`;
        const response = await this.client.post(path, JSON.stringify({ cardID, amount, intent }), { csrfProtectionToken });

        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Can not add funds',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return {
            success: result.success,
            clientSecret: result.clientSecret,
            paymentIntentID: result.paymentIntentID,
        };
    }

    /**
     * Creates a payment intent to add funds to the user's account.
     *
     * @throws Error
     */
    public async createIntent(amount: number, withCustomCard: boolean, csrfProtectionToken: string): Promise<string> {
        const path = `${this.ROOT_PATH}/create-intent`;
        const response = await this.client.post(path, JSON.stringify({ amount, withCustomCard }), { csrfProtectionToken });

        const result = await response.json();
        if (response.ok) return result.clientSecret;

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not create a payment intent',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Gets a setup intent secret to set up a card with stripe.
     *
     * @return string - the client secret for the stripe setup intent.
     * @throws Error
     */
    public async getCardSetupSecret(): Promise<string> {
        const path = `${this.ROOT_PATH}/card-setup-secret`;
        const response = await this.client.get(path);

        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Can not add card',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

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
            throw new APIError({
                status: response.status,
                message: 'Can not get account balance',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const balance = await response.json();
        if (balance) {
            return new AccountBalance(balance.freeCredits, balance.coins, balance.credits);
        }

        return new AccountBalance();
    }

    /**
     * Try to set up a payment account.
     *
     * @throws Error
     */
    public async setupAccount(csrfProtectionToken: string): Promise<string> {
        const path = `${this.ROOT_PATH}/account`;
        const response = await this.client.post(path, null, { csrfProtectionToken });
        const couponType = await response.json();

        if (response.ok) {
            return couponType;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not setup account',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * productsUsageAndCharges returns usage and how much money current user will be charged for each project which he owns split by product.
     */
    public async productsUsageAndCharges(start: Date, end: Date): Promise<ProductCharges> {
        const since = Time.toUnixTimestamp(start).toString();
        const before = Time.toUnixTimestamp(end).toString();
        const path = `${this.ROOT_PATH}/account/product-charges?from=${since}&to=${before}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get products charges',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return ProductCharges.fromJSON(await response.json());
    }

    /**
     * projectUsagePriceModel returns the user's default price model for project usage.
     */
    public async projectUsagePriceModel(): Promise<UsagePriceModel> {
        const path = `${this.ROOT_PATH}/pricing`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get project usage price model',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const model = await response.json();
        if (model) {
            return new UsagePriceModel(model.storageMBMonthCents, model.egressMBCents, model.segmentMonthCents);
        }

        return new UsagePriceModel();
    }

    /**
     * getPlacementPriceModel returns the usage price model for the user and placement.
     */
    public async getPlacementPriceModel(params: PriceModelForPlacementRequest): Promise<UsagePriceModel> {
        const url = new URL(`${this.ROOT_PATH}/placement-pricing`, window.location.href);
        url.searchParams.append('projectID', params.projectID);
        if (params.placementName) {
            url.searchParams.append('placementName', params.placementName);
        } else if (params.placement) {
            url.searchParams.append('placement', params.placement.toString());
        }
        const response = await this.client.get(url.toString());

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get price model for placement',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const model = await response.json();
        if (model) {
            return new UsagePriceModel(model.storageMBMonthCents, model.egressMBCents, model.segmentMonthCents);
        }

        return new UsagePriceModel();
    }

    /**
     * Add payment method.
     * @param request - the parameters to add the card with.
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async addCardByPaymentMethodID(request: AddCardRequest, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/payment-methods`;
        const response = await this.client.post(path, JSON.stringify(request), { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error || 'Can not add payment method',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Attempt to pay overdue invoices.
     */
    public async attemptPayments(csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/attempt-payments`;
        const response = await this.client.post(path, null, { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error || 'Can not attempt payments.',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Update credit card
     * @param params - the parameters to update the card with.
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async updateCreditCard(params: UpdateCardParams, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.put(path, JSON.stringify(params), { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not update credit card',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Detach credit card from payment account.
     *
     * @param cardId
     * @param csrfProtectionToken
     * @throws Error
     */
    public async removeCreditCard(cardId: string, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards/${cardId}`;
        const response = await this.client.delete(path, null, { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not remove credit card',
            requestID: response.headers.get('x-request-id'),
        });
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
            throw new APIError({
                status: response.status,
                message: 'can not list credit cards',
                requestID: response.headers.get('x-request-id'),
            });
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
     * @param csrfProtectionToken
     * @throws Error
     */
    public async makeCreditCardDefault(cardId: string, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.patch(path, cardId, { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not make credit card default',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Returns a list of invoices, transactions and all others payments history items for payment account.
     *
     * @returns list of payments history items
     * @throws Error
     */
    public async paymentsHistory(param: PaymentHistoryParam): Promise<PaymentHistoryPage> {
        let path = `${this.ROOT_PATH}/invoice-history?limit=${param.limit}`;
        if (param.startingAfter) {
            path = `${path}&starting_after=${param.startingAfter}`;
        } else if (param.endingBefore) {
            path = `${path}&ending_before=${param.endingBefore}`;
        }
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not list billing history',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const pageJson = await response.json();
        let items: PaymentsHistoryItem[] = [];
        if (pageJson.items) {
            items = pageJson.items.map(item =>
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

        return new PaymentHistoryPage(
            items,
            pageJson.next,
            pageJson.previous,
        );
    }

    /**
     * Returns a list of native token payments.
     *
     * @returns list of native token payment history items
     * @throws Error
     */
    public async nativePaymentsHistory(): Promise<NativePaymentHistoryItem[]> {
        const path = `${this.ROOT_PATH}/wallet/payments`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not list token payment history',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const json = await response.json();
        if (!json) return [];
        if (json.payments) {
            return json.payments.map(item =>
                new NativePaymentHistoryItem(
                    item.ID,
                    item.Wallet,
                    item.Type,
                    new TokenAmount(item.Amount.value, item.Amount.currency),
                    new TokenAmount(item.Received.value, item.Received.currency),
                    item.Status,
                    item.Link,
                    new Date(item.Timestamp),
                ),
            );
        }

        return [];
    }

    /**
     * Returns a list of STORJ token payments with confirmations.
     *
     * @returns list of native token payment items with confirmations
     * @throws Error
     */
    public async paymentsWithConfirmations(): Promise<PaymentWithConfirmations[]> {
        const path = `${this.ROOT_PATH}/wallet/payments-with-confirmations`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('Can not list token payment with confirmations');
        }

        const json = await response.json();
        if (json && json.length) {
            return json.map(item =>
                new PaymentWithConfirmations(
                    item.to,
                    parseFloat(item.tokenValue),
                    parseFloat(item.usdValue),
                    item.transaction,
                    new Date(item.timestamp),
                    parseFloat(item.bonusTokens),
                    item.status,
                    item.confirmations,
                ),
            );
        }

        return [];
    }

    /**
     * applyCouponCode applies a coupon code.
     *
     * @param couponCode
     * @param csrfProtectionToken
     * @throws Error
     */
    public async applyCouponCode(couponCode: string, csrfProtectionToken: string): Promise<Coupon> {
        const path = `${this.ROOT_PATH}/coupon/apply`;
        const response = await this.client.patch(path, couponCode, { csrfProtectionToken });

        const requestID = response.headers.get('x-request-id');
        let errMsg = `Could not apply coupon code "${couponCode}"`;

        if (!response.ok) {
            if (response.status === 409) {
                errMsg = 'You currently have an active coupon. Please try again when your coupon is no longer active, or contact Support for further help.';
            } else if (response.status === 429) {
                errMsg = 'You\'ve exceeded limit of attempts, try again in 5 minutes';
            }

            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: requestID,
            });
        }

        const coupon = await response.json();

        if (!coupon) {
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: requestID,
            });
        }

        return new Coupon(
            coupon.id,
            coupon.promoCode,
            coupon.name,
            coupon.amountOff,
            coupon.percentOff,
            new Date(coupon.addedAt),
            coupon.expiresAt ? new Date(coupon.expiresAt) : null,
            coupon.duration,
            coupon.partnered,
        );
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
            throw new APIError({
                status: response.status,
                message: 'Can not retrieve coupon',
                requestID: response.headers.get('x-request-id'),
            });
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
            coupon.duration,
            coupon.partnered,
        );
    }

    /**
     * Get native storj token wallet.
     *
     * @returns wallet
     * @throws Error
     */
    public async getWallet(): Promise<Wallet> {
        const path = `${this.ROOT_PATH}/wallet`;
        const response = await this.client.get(path);

        if (!response.ok) {
            switch (response.status) {
            case 404:
                return new Wallet();
            default:
                throw new APIError({
                    status: response.status,
                    message: 'Can not get wallet',
                    requestID: response.headers.get('x-request-id'),
                });
            }
        }

        const wallet = await response.json();
        if (wallet) {
            return new Wallet(wallet.address, new TokenAmount(wallet.balance.value, wallet.balance.currency));
        }

        throw new Error('Can not get wallet');
    }

    /**
     * get user's billing information.
     *
     * @throws Error
     */
    public async getBillingInformation(): Promise<BillingInformation> {
        const path = `${this.ROOT_PATH}/account/billing-information`;
        const response = await this.client.get(path);

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Could not get billing information',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * add user's default invoice reference.
     *
     * @param reference - invoice reference to be shown on invoices
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async addInvoiceReference(reference: string, csrfProtectionToken: string): Promise<BillingInformation> {
        const path = `${this.ROOT_PATH}/account/invoice-reference`;
        const response = await this.client.post(path, JSON.stringify({ reference }), { csrfProtectionToken });

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Could not save billing information',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * save user's billing information.
     *
     * @param address - billing information to save
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async saveBillingAddress(address: BillingAddress, csrfProtectionToken: string): Promise<BillingInformation> {
        const path = `${this.ROOT_PATH}/account/billing-address`;
        const response = await this.client.patch(path, JSON.stringify(address), { csrfProtectionToken });

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Could not save billing information',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * Claim new native storj token wallet.
     *
     * @returns wallet
     * @throws Error
     */
    public async claimWallet(csrfProtectionToken: string): Promise<Wallet> {
        const path = `${this.ROOT_PATH}/wallet`;
        const response = await this.client.post(path, null, { csrfProtectionToken });

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not claim wallet',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const wallet = await response.json();
        if (wallet) {
            return new Wallet(wallet.address, wallet.balance);
        }

        return new Wallet();
    }

    /**
     * get a list of countries whose taxes are supported.
     *
     * @throws Error
     */
    public async getTaxCountries(): Promise<TaxCountry[]> {
        const path = `${this.ROOT_PATH}/countries`;
        const response = await this.client.get(path);

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Could not countries',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * get a list of supported taxes for a country.
     *
     * @throws Error
     */
    public async getCountryTaxes(countryCode: string): Promise<TaxCountry[]> {
        const path = `${this.ROOT_PATH}/countries/${countryCode}/taxes`;
        const response = await this.client.get(path);

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || `Could not get ${countryCode} taxes`,
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * add a tax ID to a user's account.
     *
     * @param type - tax ID type
     * @param value - tax ID value
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async addTaxID(type: string, value: string, csrfProtectionToken: string): Promise<BillingInformation> {
        const path = `${this.ROOT_PATH}/account/tax-ids`;
        const response = await this.client.post(path, JSON.stringify({ type, value }), { csrfProtectionToken });

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Could not add tax ID information',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * remove a tax ID from a user's account.
     *
     * @throws Error
     */
    public async removeTaxID(taxID: string, csrfProtectionToken: string): Promise<BillingInformation> {
        const path = `${this.ROOT_PATH}/account/tax-ids/${taxID}`;
        const response = await this.client.delete(path, null, { csrfProtectionToken });

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Could not add tax ID information',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }

    /**
     * Purchases makes a purchase using a credit card action.
     * Used for pricing packages and upgrade account.
     *
     * @param request - purchase request
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async purchase(request: PurchaseRequest, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/purchase`;
        const response = await this.client.post(path, JSON.stringify(request), { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not process purchase action',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Returns whether there is a pricing package configured for the user's partner.
     *
     * @throws Error
     */
    public async pricingPackageAvailable(): Promise<boolean> {
        const path = `${this.ROOT_PATH}/package-available`;
        const response = await this.client.get(path);

        if (response.ok) {
            return await response.json();
        }

        throw new APIError({
            status: response.status,
            message: 'Could not check pricing package availability',
            requestID: response.headers.get('x-request-id'),
        });
    }

    public async startFreeTrial(csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/start-trial`;
        const response = await this.client.post(path, null, { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not start free trial',
            requestID: response.headers.get('x-request-id'),
        });
    }
}
