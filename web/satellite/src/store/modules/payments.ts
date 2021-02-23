// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import {
    AccountBalance,
    CreditCard,
    DateRange,
    PaymentsApi,
    PaymentsHistoryItem,
    PaymentsHistoryItemStatus,
    PaymentsHistoryItemType,
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
    SET_PAYMENTS_HISTORY: 'SET_PAYMENTS_HISTORY',
    SET_PROJECT_USAGE_AND_CHARGES: 'SET_PROJECT_USAGE_AND_CHARGES',
    SET_CURRENT_ROLLUP_PRICE: 'SET_CURRENT_ROLLUP_PRICE',
    SET_PREVIOUS_ROLLUP_PRICE: 'SET_PREVIOUS_ROLLUP_PRICE',
    SET_PRICE_SUMMARY: 'SET_PRICE_SUMMARY',
    SET_PRICE_SUMMARY_FOR_SELECTED_PROJECT: 'SET_PRICE_SUMMARY_FOR_SELECTED_PROJECT',
    SET_PAYWALL_ENABLED_STATUS: 'SET_PAYWALL_ENABLED_STATUS',
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
    GET_PAYMENTS_HISTORY: 'getPaymentsHistory',
    MAKE_TOKEN_DEPOSIT: 'makeTokenDeposit',
    GET_PROJECT_USAGE_AND_CHARGES: 'getProjectUsageAndCharges',
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP: 'getProjectUsageAndChargesCurrentRollup',
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP: 'getProjectUsageAndChargesPreviousRollup',
    GET_PAYWALL_ENABLED_STATUS: 'getPaywallEnabledStatus',
};

const {
    SET_BALANCE,
    SET_CREDIT_CARDS,
    SET_DATE,
    CLEAR,
    UPDATE_CARDS_SELECTION,
    UPDATE_CARDS_DEFAULT,
    SET_PAYMENTS_HISTORY,
    SET_PROJECT_USAGE_AND_CHARGES,
    SET_PRICE_SUMMARY,
    SET_PRICE_SUMMARY_FOR_SELECTED_PROJECT,
    SET_PAYWALL_ENABLED_STATUS,
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
    GET_PAYMENTS_HISTORY,
    MAKE_TOKEN_DEPOSIT,
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP,
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP,
    GET_PAYWALL_ENABLED_STATUS,
} = PAYMENTS_ACTIONS;

export class PaymentsState {
    /**
     * balance stores in cents
     */
    public balance: AccountBalance = new AccountBalance();
    public creditCards: CreditCard[] = [];
    public paymentsHistory: PaymentsHistoryItem[] = [];
    public usageAndCharges: ProjectUsageAndCharges[] = [];
    public priceSummary: number = 0;
    public priceSummaryForSelectedProject: number = 0;
    public startDate: Date = new Date();
    public endDate: Date = new Date();
    public isPaywallEnabled: boolean = true;
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
            [SET_BALANCE](state: PaymentsState, balance: AccountBalance): void {
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
            [SET_PAYMENTS_HISTORY](state: PaymentsState, paymentsHistory: PaymentsHistoryItem[]): void {
                state.paymentsHistory = paymentsHistory;
            },
            [SET_PROJECT_USAGE_AND_CHARGES](state: PaymentsState, usageAndCharges: ProjectUsageAndCharges[]): void {
                state.usageAndCharges = usageAndCharges;
            },
            [SET_PRICE_SUMMARY](state: PaymentsState, charges: ProjectUsageAndCharges[]): void {
                if (charges.length === 0) {
                    state.priceSummary = 0;

                    return;
                }

                const usageItemSummaries: number[] = charges.map(item => item.summary());

                state.priceSummary = usageItemSummaries.reduce((accumulator, current) => accumulator + current);
            },
            [SET_PRICE_SUMMARY_FOR_SELECTED_PROJECT](state: PaymentsState, selectedProjectId: string): void {
                let usageAndChargesForSelectedProject: ProjectUsageAndCharges | undefined;
                if (state.usageAndCharges.length) {
                    usageAndChargesForSelectedProject = state.usageAndCharges.find(item => item.projectId === selectedProjectId);
                }

                if (!usageAndChargesForSelectedProject) {
                    state.priceSummaryForSelectedProject = 0;

                    return;
                }

                state.priceSummaryForSelectedProject = usageAndChargesForSelectedProject.summary();
            },
            [SET_PAYWALL_ENABLED_STATUS](state: PaymentsState, isPaywallEnabled: boolean): void {
                state.isPaywallEnabled = isPaywallEnabled;
            },
            [CLEAR](state: PaymentsState) {
                state.balance = new AccountBalance();
                state.paymentsHistory = [];
                state.usageAndCharges = [];
                state.priceSummary = 0;
                state.creditCards = [];
                state.startDate = new Date();
                state.endDate = new Date();
                state.isPaywallEnabled = true;
            },
        },
        actions: {
            [GET_BALANCE]: async function({commit}: any): Promise<AccountBalance> {
                const balance: AccountBalance = await api.getBalance();

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
            [GET_PAYMENTS_HISTORY]: async function({commit}: any): Promise<void> {
                const paymentsHistory: PaymentsHistoryItem[] = await api.paymentsHistory();

                commit(SET_PAYMENTS_HISTORY, paymentsHistory);
            },
            [MAKE_TOKEN_DEPOSIT]: async function({commit}: any, amount: number): Promise<TokenDeposit> {
                return await api.makeTokenDeposit(amount);
            },
            [GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP]: async function({commit, rootGetters}: any): Promise<void> {
                const now = new Date();
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0));

                const usageAndCharges: ProjectUsageAndCharges[] = await api.projectsUsageAndCharges(startUTC, endUTC);

                commit(SET_DATE, new DateRange(startUTC, endUTC));
                commit(SET_PROJECT_USAGE_AND_CHARGES, usageAndCharges);
                commit(SET_PRICE_SUMMARY, usageAndCharges);
                commit(SET_PRICE_SUMMARY_FOR_SELECTED_PROJECT, rootGetters.selectedProject.id);
            },
            [GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP]: async function({commit}: any): Promise<void> {
                const now = new Date();
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1, 0, 0));
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0, 23, 59, 59));

                const usageAndCharges: ProjectUsageAndCharges[] = await api.projectsUsageAndCharges(startUTC, endUTC);

                commit(SET_DATE, new DateRange(startUTC, endUTC));
                commit(SET_PROJECT_USAGE_AND_CHARGES, usageAndCharges);
                commit(SET_PRICE_SUMMARY, usageAndCharges);
            },
            [GET_PAYWALL_ENABLED_STATUS]: async function({commit, rootGetters}: any): Promise<void> {
                const isPaywallEnabled: boolean = await api.getPaywallStatus(rootGetters.user.id);

                commit(SET_PAYWALL_ENABLED_STATUS, isPaywallEnabled);
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
