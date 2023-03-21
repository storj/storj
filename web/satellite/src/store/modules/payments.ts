// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccountBalance,
    Coupon,
    CreditCard,
    DateRange,
    PaymentsApi,
    PaymentsHistoryItem,
    PaymentsHistoryItemStatus,
    PaymentsHistoryItemType,
    ProjectCharges,
    ProjectUsagePriceModel,
    NativePaymentHistoryItem,
    Wallet,
} from '@/types/payments';
import { StoreModule } from '@/types/store';

export const PAYMENTS_MUTATIONS = {
    SET_BALANCE: 'SET_BALANCE',
    SET_WALLET: 'SET_WALLET',
    SET_CREDIT_CARDS: 'SET_CREDIT_CARDS',
    SET_DATE: 'SET_DATE',
    CLEAR: 'CLEAR_PAYMENT_INFO',
    UPDATE_CARDS_SELECTION: 'UPDATE_CARDS_SELECTION',
    UPDATE_CARDS_DEFAULT: 'UPDATE_CARDS_DEFAULT',
    SET_PAYMENTS_HISTORY: 'SET_PAYMENTS_HISTORY',
    SET_NATIVE_PAYMENTS_HISTORY: 'SET_NATIVE_PAYMENTS_HISTORY',
    SET_PROJECT_USAGE_AND_CHARGES: 'SET_PROJECT_USAGE_AND_CHARGES',
    SET_PROJECT_USAGE_PRICE_MODEL: 'SET_PROJECT_USAGE_PRICE_MODEL',
    SET_CURRENT_ROLLUP_PRICE: 'SET_CURRENT_ROLLUP_PRICE',
    SET_PREVIOUS_ROLLUP_PRICE: 'SET_PREVIOUS_ROLLUP_PRICE',
    SET_COUPON: 'SET_COUPON',
};

export const PAYMENTS_ACTIONS = {
    GET_BALANCE: 'getBalance',
    GET_WALLET: 'getWallet',
    CLAIM_WALLET: 'claimWallet',
    SETUP_ACCOUNT: 'setupAccount',
    GET_CREDIT_CARDS: 'getCreditCards',
    ADD_CREDIT_CARD: 'addCreditCard',
    CLEAR_PAYMENT_INFO: 'clearPaymentInfo',
    TOGGLE_CARD_SELECTION: 'toggleCardSelection',
    CLEAR_CARDS_SELECTION: 'clearCardsSelection',
    MAKE_CARD_DEFAULT: 'makeCardDefault',
    REMOVE_CARD: 'removeCard',
    GET_PAYMENTS_HISTORY: 'getPaymentsHistory',
    GET_NATIVE_PAYMENTS_HISTORY: 'getNativePaymentsHistory',
    GET_PROJECT_USAGE_AND_CHARGES: 'getProjectUsageAndCharges',
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP: 'getProjectUsageAndChargesCurrentRollup',
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP: 'getProjectUsageAndChargesPreviousRollup',
    GET_PROJECT_USAGE_PRICE_MODEL: 'getProjectUsagePriceModel',
    APPLY_COUPON_CODE: 'applyCouponCode',
    GET_COUPON: `getCoupon`,
    PURCHASE_PACKAGE: 'purchasePackage',
};

const {
    SET_BALANCE,
    SET_WALLET,
    SET_CREDIT_CARDS,
    SET_DATE,
    CLEAR,
    UPDATE_CARDS_SELECTION,
    UPDATE_CARDS_DEFAULT,
    SET_PAYMENTS_HISTORY,
    SET_NATIVE_PAYMENTS_HISTORY,
    SET_PROJECT_USAGE_AND_CHARGES,
    SET_PROJECT_USAGE_PRICE_MODEL: SET_PROJECT_USAGE_PRICE_MODEL,
    SET_COUPON,
} = PAYMENTS_MUTATIONS;

const {
    GET_BALANCE,
    GET_WALLET,
    CLAIM_WALLET,
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
    ADD_CREDIT_CARD,
    TOGGLE_CARD_SELECTION,
    CLEAR_CARDS_SELECTION,
    CLEAR_PAYMENT_INFO,
    MAKE_CARD_DEFAULT,
    REMOVE_CARD,
    GET_PAYMENTS_HISTORY,
    GET_NATIVE_PAYMENTS_HISTORY,
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP,
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP,
    GET_PROJECT_USAGE_PRICE_MODEL: GET_PROJECT_USAGE_PRICE_MODEL,
    APPLY_COUPON_CODE,
    GET_COUPON,
    PURCHASE_PACKAGE,
} = PAYMENTS_ACTIONS;

export class PaymentsState {
    /**
     * balance stores in cents
     */
    public balance: AccountBalance = new AccountBalance();
    public creditCards: CreditCard[] = [];
    public paymentsHistory: PaymentsHistoryItem[] = [];
    public nativePaymentsHistory: NativePaymentHistoryItem[] = [];
    public projectCharges: ProjectCharges = new ProjectCharges();
    public usagePriceModel: ProjectUsagePriceModel = new ProjectUsagePriceModel();
    public startDate: Date = new Date();
    public endDate: Date = new Date();
    public coupon: Coupon | null = null;
    public wallet: Wallet = new Wallet();
}

interface PaymentsContext {
    state: PaymentsState
    commit: (string, ...unknown) => void
    rootGetters: {
        selectedProject: {
            id: string
        }
    }
}

/**
 * creates payments module with all dependencies
 *
 * @param api - payments api
 */
export function makePaymentsModule(api: PaymentsApi): StoreModule<PaymentsState, PaymentsContext> {
    return {
        state: new PaymentsState(),
        mutations: {
            [SET_BALANCE](state: PaymentsState, balance: AccountBalance): void {
                state.balance = balance;
            },
            [SET_WALLET](state: PaymentsState, wallet: Wallet): void {
                state.wallet = wallet;
            },
            [SET_CREDIT_CARDS](state: PaymentsState, creditCards: CreditCard[]): void {
                state.creditCards = creditCards;
            },
            [SET_DATE](state: PaymentsState, dateRange: DateRange) {
                state.startDate = dateRange.startDate;
                state.endDate = dateRange.endDate;
            },
            [UPDATE_CARDS_SELECTION](state: PaymentsState, id: string | null): void {
                state.creditCards = state.creditCards.map(card => {
                    if (card.id === id) {
                        card.isSelected = !card.isSelected;

                        return card;
                    }

                    card.isSelected = false;

                    return card;
                });
            },
            [UPDATE_CARDS_DEFAULT](state: PaymentsState, id: string): void {
                state.creditCards = state.creditCards.map(card => {
                    if (card.id === id) {
                        card.isDefault = !card.isDefault;

                        return card;
                    }

                    card.isDefault = false;

                    return card;
                });
            },
            [SET_PAYMENTS_HISTORY](state: PaymentsState, paymentsHistory: PaymentsHistoryItem[]): void {
                state.paymentsHistory = paymentsHistory;
            },
            [SET_NATIVE_PAYMENTS_HISTORY](state: PaymentsState, paymentsHistory: NativePaymentHistoryItem[]): void {
                state.nativePaymentsHistory = paymentsHistory;
            },
            [SET_PROJECT_USAGE_AND_CHARGES](state: PaymentsState, projectPartnerCharges: ProjectCharges): void {
                state.projectCharges = projectPartnerCharges;
            },
            [SET_PROJECT_USAGE_PRICE_MODEL](state: PaymentsState, model: ProjectUsagePriceModel): void {
                state.usagePriceModel = model;
            },
            [SET_COUPON](state: PaymentsState, coupon: Coupon): void {
                state.coupon = coupon;
            },
            [CLEAR](state: PaymentsState) {
                state.balance = new AccountBalance();
                state.creditCards = [];
                state.paymentsHistory = [];
                state.nativePaymentsHistory = [];
                state.projectCharges = new ProjectCharges();
                state.usagePriceModel = new ProjectUsagePriceModel();
                state.startDate = new Date();
                state.endDate = new Date();
                state.coupon = null;
                state.wallet = new Wallet();
            },
        },
        actions: {
            [GET_BALANCE]: async function({ commit }: PaymentsContext): Promise<AccountBalance> {
                const balance: AccountBalance = await api.getBalance();

                commit(SET_BALANCE, balance);

                return balance;
            },
            [GET_WALLET]: async function({ commit }: PaymentsContext): Promise<void> {
                const wallet: Wallet = await api.getWallet();

                commit(SET_WALLET, wallet);
            },
            [CLAIM_WALLET]: async function({ commit }: PaymentsContext): Promise<void> {
                const wallet: Wallet = await api.claimWallet();

                commit(SET_WALLET, wallet);
            },
            [SETUP_ACCOUNT]: async function(): Promise<string> {
                const couponType = await api.setupAccount();

                return couponType;
            },
            [GET_CREDIT_CARDS]: async function({ commit }: PaymentsContext): Promise<CreditCard[]> {
                const creditCards = await api.listCreditCards();

                commit(SET_CREDIT_CARDS, creditCards);

                return creditCards;
            },
            [ADD_CREDIT_CARD]: async function(_context: PaymentsContext, token: string): Promise<void> {
                await api.addCreditCard(token);
            },
            [TOGGLE_CARD_SELECTION]: function({ commit }: PaymentsContext, id: string): void {
                commit(UPDATE_CARDS_SELECTION, id);
            },
            [CLEAR_CARDS_SELECTION]: function({ commit }: PaymentsContext): void {
                commit(UPDATE_CARDS_SELECTION, null);
            },
            [MAKE_CARD_DEFAULT]: async function({ commit }: PaymentsContext, id: string): Promise<void> {
                await api.makeCreditCardDefault(id);

                commit(UPDATE_CARDS_DEFAULT, id);
            },
            [REMOVE_CARD]: async function({ commit, state }: PaymentsContext, cardId: string): Promise<void> {
                await api.removeCreditCard(cardId);

                commit(SET_CREDIT_CARDS, state.creditCards.filter(card => card.id !== cardId));
            },
            [CLEAR_PAYMENT_INFO]: function({ commit }: PaymentsContext): void {
                commit(CLEAR);
            },
            [GET_PAYMENTS_HISTORY]: async function({ commit }: PaymentsContext): Promise<void> {
                const paymentsHistory = await api.paymentsHistory();

                commit(SET_PAYMENTS_HISTORY, paymentsHistory);
            },
            [GET_NATIVE_PAYMENTS_HISTORY]: async function({ commit }: PaymentsContext): Promise<void> {
                const paymentsHistory = await api.nativePaymentsHistory();

                commit(SET_NATIVE_PAYMENTS_HISTORY, paymentsHistory);
            },
            [GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP]: async function({ commit, rootGetters }: PaymentsContext): Promise<void> {
                const now = new Date();
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0));

                const projectPartnerCharges: ProjectCharges = await api.projectsUsageAndCharges(startUTC, endUTC);

                commit(SET_DATE, new DateRange(startUTC, endUTC));
                commit(SET_PROJECT_USAGE_AND_CHARGES, projectPartnerCharges);
            },
            [GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP]: async function({ commit }: PaymentsContext): Promise<void> {
                const now = new Date();
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1, 0, 0));
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0, 23, 59, 59));

                const projectPartnerCharges: ProjectCharges = await api.projectsUsageAndCharges(startUTC, endUTC);

                commit(SET_DATE, new DateRange(startUTC, endUTC));
                commit(SET_PROJECT_USAGE_AND_CHARGES, projectPartnerCharges);
            },
            [GET_PROJECT_USAGE_PRICE_MODEL]: async function({ commit }: PaymentsContext): Promise<void> {
                const model: ProjectUsagePriceModel = await api.projectUsagePriceModel();
                commit(SET_PROJECT_USAGE_PRICE_MODEL, model);
            },
            [APPLY_COUPON_CODE]: async function({ commit }: PaymentsContext, code: string): Promise<void> {
                const coupon = await api.applyCouponCode(code);
                commit(SET_COUPON, coupon);
            },
            [GET_COUPON]: async function({ commit }: PaymentsContext): Promise<void> {
                const coupon = await api.getCoupon();
                commit(SET_COUPON, coupon);
            },
            [PURCHASE_PACKAGE]: async function(_: PaymentsContext, token: string): Promise<void> {
                await api.purchasePricingPackage(token);
            },
        },
        getters: {
            canUserCreateFirstProject: (state: PaymentsState): boolean => {
                return state.balance.sum > 0 || state.creditCards.length > 0;
            },
            isTransactionProcessing: (state: PaymentsState): boolean => {
                return state.paymentsHistory.some((paymentsItem: PaymentsHistoryItem) => {
                    return paymentsItem.amount >= 50 && paymentsItem.type === PaymentsHistoryItemType.Transaction
                        && (paymentsItem.status === PaymentsHistoryItemStatus.Pending
                        || paymentsItem.status === PaymentsHistoryItemStatus.Paid
                        || paymentsItem.status === PaymentsHistoryItemStatus.Completed);
                }) && state.balance.sum === 0;
            },
            isBalancePositive: (state: PaymentsState): boolean => {
                return state.balance.sum > 0;
            },
        },
    };
}
