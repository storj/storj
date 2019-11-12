// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import {BillingHistoryItem, CreditCard, DepositInfo, PaymentsApi} from '@/types/payments';

const PAYMENTS_MUTATIONS = {
    SET_BALANCE: 'SET_BALANCE',
    SET_CREDIT_CARDS: 'SET_CREDIT_CARDS',
    CLEAR: 'CLEAR_PAYMENT_INFO',
    UPDATE_CARDS_SELECTION: 'UPDATE_CARDS_SELECTION',
    UPDATE_CARDS_DEFAULT: 'UPDATE_CARDS_DEFAULT',
    SET_BILLING_HISTORY: 'SET_BILLING_HISTORY',
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
};

const {
    SET_BALANCE,
    SET_CREDIT_CARDS,
    CLEAR,
    UPDATE_CARDS_SELECTION,
    UPDATE_CARDS_DEFAULT,
    SET_BILLING_HISTORY,
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
} = PAYMENTS_ACTIONS;

export class PaymentsState {
    /**
     * balance stores in cents
     */
    public balance: number = 0;
    public creditCards: CreditCard[] = [];
    public billingHistory: BillingHistoryItem[] = [];
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
            [CLEAR](state: PaymentsState) {
                state.balance = 0;
                state.creditCards = [];
            },
        },
        actions: {
            [GET_BALANCE]: async function({commit}: any): Promise<void> {
                const balance = await api.getBalance();

                commit(SET_BALANCE, balance);
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
            [MAKE_TOKEN_DEPOSIT]: async function({commit}: any, amount: string): Promise<DepositInfo> {
                return await api.makeTokenDeposit(amount);
            },
        },
    };
}
