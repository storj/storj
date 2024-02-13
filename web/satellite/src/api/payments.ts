// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorConflict } from './errors/ErrorConflict';
import { ErrorTooManyRequests } from './errors/ErrorTooManyRequests';

import {
    AccountBalance,
    Coupon,
    CreditCard,
    PaymentsApi,
    PaymentsHistoryItem,
    ProjectCharges,
    ProjectUsagePriceModel,
    TokenAmount,
    NativePaymentHistoryItem,
    Wallet,
    PaymentWithConfirmations, PaymentHistoryParam, PaymentHistoryPage,
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
    public async setupAccount(): Promise<string> {
        const path = `${this.ROOT_PATH}/account`;
        const response = await this.client.post(path, null);
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
     * projectsUsageAndCharges returns usage and how much money current user will be charged for each project which he owns.
     */
    public async projectsUsageAndCharges(start: Date, end: Date): Promise<ProjectCharges> {
        const since = Time.toUnixTimestamp(start).toString();
        const before = Time.toUnixTimestamp(end).toString();
        const path = `${this.ROOT_PATH}/account/charges?from=${since}&to=${before}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get projects charges',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return ProjectCharges.fromJSON(await response.json());
    }

    /**
     * projectUsagePriceModel returns the user's default price model for project usage.
     */
    public async projectUsagePriceModel(): Promise<ProjectUsagePriceModel> {
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
            return new ProjectUsagePriceModel(model.storageMBMonthCents, model.egressMBCents, model.segmentMonthCents);
        }

        return new ProjectUsagePriceModel();
    }

    /**
     * Add payment method.
     * @param pmID - stripe payment method id of the credit card
     * @throws Error
     */
    public async addCardByPaymentMethodID(pmID: string): Promise<void> {
        const path = `${this.ROOT_PATH}/payment-methods`;
        const response = await this.client.post(path, pmID);

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

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not add credit card',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Detach credit card from payment account.
     *
     * @param cardId
     * @throws Error
     */
    public async removeCreditCard(cardId: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards/${cardId}`;
        const response = await this.client.delete(path, null);

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
     * @throws Error
     */
    public async makeCreditCardDefault(cardId: string): Promise<void> {
        const path = `${this.ROOT_PATH}/cards`;
        const response = await this.client.patch(path, cardId);

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
     * @throws Error
     */
    public async applyCouponCode(couponCode: string): Promise<Coupon> {
        const path = `${this.ROOT_PATH}/coupon/apply`;
        const response = await this.client.patch(path, couponCode);
        const errMsg = `Could not apply coupon code "${couponCode}"`;

        if (!response.ok) {
            switch (response.status) {
            case 409:
                throw new ErrorConflict('You currently have an active coupon. Please try again when your coupon is no longer active, or contact Support for further help.');
            case 429:
                throw new ErrorTooManyRequests('You\'ve exceeded limit of attempts, try again in 5 minutes');
            default:
                throw new Error(errMsg);
            }
        }

        const coupon = await response.json();

        if (!coupon) {
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
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
            return new Wallet(wallet.address, wallet.balance);
        }

        throw new Error('Can not get wallet');
    }

    /**
     * Claim new native storj token wallet.
     *
     * @returns wallet
     * @throws Error
     */
    public async claimWallet(): Promise<Wallet> {
        const path = `${this.ROOT_PATH}/wallet`;
        const response = await this.client.post(path, null);

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
     * Purchases the pricing package associated with the user's partner.
     *
     * @param dataStr - the Stripe payment method id or token of the credit card
     * @param isPMID - whether the dataStr is a payment method id or token
     * @throws Error
     */
    public async purchasePricingPackage(dataStr: string, isPMID: boolean): Promise<void> {
        const path = `${this.ROOT_PATH}/purchase-package?pmID=${isPMID}`;
        const response = await this.client.post(path, dataStr);

        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not purchase pricing package',
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
}
