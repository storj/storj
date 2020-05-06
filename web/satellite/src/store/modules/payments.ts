// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import {
    BillingHistoryItem,
    BillingHistoryItemStatus,
    BillingHistoryItemType,
    CreditCard,
    DateRange,
    PaymentsApi,
    ProjectUsageAndCharges,
    TokenDeposit,
} from '@/types/payments';

export const PAYMENTS_MUTATIONS = {
    SET_BALANCE: 'SET_BALANCE',
    SET_CREDIT_CARDS: 'SET_CREDIT_CARDS',
    SET_DATE: 'SET_DATE',
    CLEAR: 'CLEAR_PAYMENT_INFO',
    UPDATE_CARDS_SELECTION: 'UPDATE_CARDS_SELECTION',
    UPDATE_CARDS_DEFAULT: 'UPDATE_CARDS_DEFAULT',
    SET_BILLING_HISTORY: 'SET_BILLING_HISTORY',
    SET_PROJECT_USAGE_AND_CHARGES: 'SET_PROJECT_USAGE_AND_CHARGES',
    SET_CURRENT_ROLLUP_PRICE: 'SET_CURRENT_ROLLUP_PRICE',
    SET_PREVIOUS_ROLLUP_PRICE: 'SET_PREVIOUS_ROLLUP_PRICE',
    SET_PRICE_SUMMARY: 'SET_PRICE_SUMMARY',
};

export const PAYMENTS_ACTIONS = {
    GET_BALANCE: 'getBalance',
    SETUP_ACCOUNT: 'setupAccount',
    GET_CREDIT_CARDS: 'getCreditCards',
    ADD_CREDIT_CARD: 'addCreditCard',
    CLEAR_PAYMENT_INFO: 'clearPaymentInfo',
    TOGGLE_CARD_SELECTION: 'toggleCardSelection',
    CLEAR_CARDS_SELECTION: 'clearCardsSelection',
    MAKE_CARD_DEFAULT: 'makeCardDefault',
    REMOVE_CARD: 'removeCard',
    GET_BILLING_HISTORY: 'getBillingHistory',
    MAKE_TOKEN_DEPOSIT: 'makeTokenDeposit',
    GET_PROJECT_USAGE_AND_CHARGES: 'getProjectUsageAndCharges',
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP: 'getProjectUsageAndChargesCurrentRollup',
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP: 'getProjectUsageAndChargesPreviousRollup',
};

const {
    SET_BALANCE,
    SET_CREDIT_CARDS,
    SET_DATE,
    CLEAR,
    UPDATE_CARDS_SELECTION,
    UPDATE_CARDS_DEFAULT,
    SET_BILLING_HISTORY,
    SET_PROJECT_USAGE_AND_CHARGES,
    SET_CURRENT_ROLLUP_PRICE,
    SET_PREVIOUS_ROLLUP_PRICE,
    SET_PRICE_SUMMARY,
} = PAYMENTS_MUTATIONS;

const {
    GET_BALANCE,
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
    ADD_CREDIT_CARD,
    TOGGLE_CARD_SELECTION,
    CLEAR_CARDS_SELECTION,
    CLEAR_PAYMENT_INFO,
    MAKE_CARD_DEFAULT,
    REMOVE_CARD,
    GET_BILLING_HISTORY,
    MAKE_TOKEN_DEPOSIT,
    GET_PROJECT_USAGE_AND_CHARGES,
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP,
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP,
} = PAYMENTS_ACTIONS;

export class PaymentsState {
    /**
     * balance stores in cents
     */
    public balance: number = 0;
    public creditCards: CreditCard[] = [];
    public billingHistory: BillingHistoryItem[] = [];
    public usageAndCharges: ProjectUsageAndCharges[] = [];
    public priceSummary: number = 0;
    public currentRollupPrice: number = 0;
    public previousRollupPrice: number = 0;
    public startDate: Date = new Date();
    public endDate: Date = new Date();
}

/**
 * creates payments module with all dependencies
 *
 * @param api - payments api
 */
export function makePaymentsModule(api: PaymentsApi): StoreModule<PaymentsState> {
    return {
        state: new PaymentsState(),
        mutations: {
            [SET_BALANCE](state: PaymentsState, balance: number): void {
                state.balance = balance;
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
            [SET_BILLING_HISTORY](state: PaymentsState, billingHistory: BillingHistoryItem[]): void {
                state.billingHistory = billingHistory;
            },
            [SET_PROJECT_USAGE_AND_CHARGES](state: PaymentsState, usageAndCharges: ProjectUsageAndCharges[]): void {
                state.usageAndCharges = usageAndCharges;
            },
            [SET_CURRENT_ROLLUP_PRICE](state: PaymentsState): void {
                state.currentRollupPrice = state.priceSummary;
            },
            [SET_PREVIOUS_ROLLUP_PRICE](state: PaymentsState): void {
                state.previousRollupPrice = state.priceSummary;
            },
            [SET_PRICE_SUMMARY](state: PaymentsState, charges: ProjectUsageAndCharges[]): void {
                if (charges.length === 0) {
                    state.priceSummary = 0;

                    return;
                }

                const usageItemSummaries = charges.map(item => item.summary());

                state.priceSummary = usageItemSummaries.reduce((accumulator, current) => accumulator + current);
            },
            [CLEAR](state: PaymentsState) {
                state.balance = 0;
                state.billingHistory = [];
                state.usageAndCharges = [];
                state.priceSummary = 0;
                state.previousRollupPrice = 0;
                state.currentRollupPrice = 0;
                state.creditCards = [];
                state.startDate = new Date();
                state.endDate = new Date();
            },
        },
        actions: {
            [GET_BALANCE]: async function({commit}: any): Promise<number> {
                const balance = await api.getBalance();

                commit(SET_BALANCE, balance);

                return balance;
            },
            [SETUP_ACCOUNT]: async function(): Promise<void> {
                await api.setupAccount();
            },
            [GET_CREDIT_CARDS]: async function({commit}: any): Promise<CreditCard[]> {
                const creditCards = await api.listCreditCards();

                commit(SET_CREDIT_CARDS, creditCards);

                return creditCards;
            },
            [ADD_CREDIT_CARD]: async function({commit}: any, token: string): Promise<void> {
                await api.addCreditCard(token);
            },
            [TOGGLE_CARD_SELECTION]: function({commit}: any, id: string): void {
                commit(UPDATE_CARDS_SELECTION, id);
            },
            [CLEAR_CARDS_SELECTION]: function({commit}: any): void {
                commit(UPDATE_CARDS_SELECTION, null);
            },
            [MAKE_CARD_DEFAULT]: async function({commit}: any, id: string): Promise<void> {
                await api.makeCreditCardDefault(id);

                commit(UPDATE_CARDS_DEFAULT, id);
            },
            [REMOVE_CARD]: async function({commit, state}: any, cardId: string): Promise<void> {
                await api.removeCreditCard(cardId);

                commit(SET_CREDIT_CARDS, state.creditCards.filter(card => card.id !== cardId));
            },
            [CLEAR_PAYMENT_INFO]: function({commit}: any): void {
                commit(CLEAR);
            },
            [GET_BILLING_HISTORY]: async function({commit}: any): Promise<void> {
                const billingHistory: BillingHistoryItem[] = await api.billingHistory();

                commit(SET_BILLING_HISTORY, billingHistory);
            },
            [MAKE_TOKEN_DEPOSIT]: async function({commit}: any, amount: number): Promise<TokenDeposit> {
                return await api.makeTokenDeposit(amount);
            },
            [GET_PROJECT_USAGE_AND_CHARGES]: async function({commit}: any, dateRange: DateRange): Promise<void> {
                const now = new Date();
                let beforeUTC = new Date(Date.UTC(dateRange.endDate.getUTCFullYear(), dateRange.endDate.getUTCMonth(), dateRange.endDate.getUTCDate(), 23, 59));

                if (now.getUTCFullYear() === dateRange.endDate.getUTCFullYear() &&
                    now.getUTCMonth() === dateRange.endDate.getUTCMonth() &&
                    now.getUTCDate() <= dateRange.endDate.getUTCDate()) {
                    beforeUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
                }

                const sinceUTC = new Date(Date.UTC(dateRange.startDate.getUTCFullYear(), dateRange.startDate.getUTCMonth(), dateRange.startDate.getUTCDate(), 0, 0));
                const usageAndCharges: ProjectUsageAndCharges[] = await api.projectsUsageAndCharges(sinceUTC, beforeUTC);

                commit(SET_DATE, dateRange);
                commit(SET_PROJECT_USAGE_AND_CHARGES, usageAndCharges);
                commit(SET_PRICE_SUMMARY, usageAndCharges);
            },
            [GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP]: async function({commit}: any): Promise<void> {
                const now = new Date();
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0));

                const usageAndCharges: ProjectUsageAndCharges[] = await api.projectsUsageAndCharges(startUTC, endUTC);

                commit(SET_DATE, new DateRange(startUTC, endUTC));
                commit(SET_PROJECT_USAGE_AND_CHARGES, usageAndCharges);
                commit(SET_PRICE_SUMMARY, usageAndCharges);
                commit(SET_CURRENT_ROLLUP_PRICE);
            },
            [GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP]: async function({commit}: any): Promise<void> {
                const now = new Date();
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1, 0, 0));
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0, 23, 59, 59));

                const usageAndCharges: ProjectUsageAndCharges[] = await api.projectsUsageAndCharges(startUTC, endUTC);

                commit(SET_DATE, new DateRange(startUTC, endUTC));
                commit(SET_PROJECT_USAGE_AND_CHARGES, usageAndCharges);
                commit(SET_PRICE_SUMMARY, usageAndCharges);
                commit(SET_PREVIOUS_ROLLUP_PRICE);
            },
        },
        getters: {
            canUserCreateFirstProject: (state: PaymentsState): boolean => {
                return (state.billingHistory.some((billingItem: BillingHistoryItem) => {
                    return billingItem.amount >= 50 && billingItem.type === BillingHistoryItemType.Transaction
                        && billingItem.status === BillingHistoryItemStatus.Completed;
                }) && state.balance > 0) || state.creditCards.length > 0;
            },
            isTransactionProcessing: (state: PaymentsState): boolean => {
                return state.billingHistory.some((billingItem: BillingHistoryItem) => {
                    return billingItem.amount >= 50 && billingItem.type === BillingHistoryItemType.Transaction
                        && (billingItem.status === BillingHistoryItemStatus.Pending
                        || billingItem.status === BillingHistoryItemStatus.Paid
                        || billingItem.status === BillingHistoryItemStatus.Completed);
                }) && state.balance === 0;
            },
            isTransactionCompleted: (state: PaymentsState): boolean => {
                return (state.billingHistory.some((billingItem: BillingHistoryItem) => {
                    return billingItem.amount >= 50 && billingItem.type === BillingHistoryItemType.Transaction
                        && billingItem.status === BillingHistoryItemStatus.Completed;
                }) && state.balance > 0);
            },
            isInvoiceForPreviousRollup: (state: PaymentsState): boolean => {
                const now = new Date();

                return state.billingHistory.some((billingItem: BillingHistoryItem) => {
                    if (now.getUTCMonth() === 0) {
                        return billingItem.type === BillingHistoryItemType.Invoice
                            && billingItem.start.getUTCFullYear() === now.getUTCFullYear() - 1
                            && billingItem.start.getUTCMonth() === 11;
                    }

                    return billingItem.type === BillingHistoryItemType.Invoice
                        && billingItem.start.getUTCFullYear() === now.getUTCFullYear()
                        && billingItem.start.getUTCMonth() === now.getUTCMonth() - 1;
                });
            },
        },
    };
}
