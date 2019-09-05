// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { CreditsApi, CreditUsage } from '@/types/credits';

export const CREDIT_USAGE_ACTIONS = {
    FETCH: 'fetchCreditUsage',
    CLEAR: 'clearCreditUsage',
};

export const CREDIT_USAGE_MUTATIONS = {
    SET: 'SET_CREDIT_USAGE',
    CLEAR: 'CLEAR_CREDIT_USAGE',
};

const { FETCH } = CREDIT_USAGE_ACTIONS;
const { SET, CLEAR } = CREDIT_USAGE_MUTATIONS;

export function makeCreditsModule(api: CreditsApi): StoreModule<CreditUsage> {
    return {
        state: new CreditUsage(),

        mutations: {
            [SET](state: CreditUsage, creditUsage: CreditUsage) {
                state.availableCredits = creditUsage.availableCredits;
                state.usedCredits = creditUsage.usedCredits;
                state.referred = creditUsage.referred;
            },
            [CLEAR](state: CreditUsage): void {
                state.availableCredits = 0;
                state.usedCredits = 0;
                state.referred = 0;
            },
        },

        actions: {
            [FETCH]: async function({commit}: any): Promise<CreditUsage> {
                const credits = await api.get();

                commit(SET, credits);

                return credits;
            },
            [CREDIT_USAGE_ACTIONS.CLEAR]: function({commit}: any): void {
                commit(CLEAR);
            },
        },

        getters: {
            credits: (state: CreditUsage): CreditUsage => state,
        },
    };
}
