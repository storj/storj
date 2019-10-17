// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { CreditCard, PaymentsApi } from '@/types/payments';

const PAYMENTS_MUTATIONS = {
    SET_BALANCE: 'SET_BALANCE',
    SET_CREDIT_CARDS: 'SET_CREDIT_CARDS',
    CLEAR: 'CLEAR_PAYMENT_INFO',
};

export const PAYMENTS_ACTIONS = {
    GET_BALANCE: 'getBalance',
    SETUP_ACCOUNT: 'setupAccount',
    GET_CREDIT_CARDS: 'getCreditCards',
    ADD_CREDIT_CARD: 'addCreditCard',
    CLEAR_PAYMENT_INFO: 'clearPaymentInfo',
};

const {
    SET_BALANCE,
    SET_CREDIT_CARDS,
    CLEAR,
} = PAYMENTS_MUTATIONS;

const {
    GET_BALANCE,
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
    ADD_CREDIT_CARD,
    CLEAR_PAYMENT_INFO,
} = PAYMENTS_ACTIONS;

class PaymentsState {
    /**
     * balance stores in cents
     */
    public balance: number = 0;
    public creditCards: CreditCard[] = [];
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
            [SET_BALANCE](state: PaymentsState, balance: number) {
                state.balance = balance;
            },
            [SET_BALANCE](state: PaymentsState, creditCards: CreditCard[]) {
                state.creditCards = creditCards;
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
            [CLEAR_PAYMENT_INFO]: function({commit}: any): void {
                commit(CLEAR);
            },
        },
        getters: {

        },
    };
}
