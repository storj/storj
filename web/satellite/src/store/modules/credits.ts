// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { CREDIT_USAGE_ACTIONS } from '@/utils/constants/actionNames';
import { StoreModule } from '@/store';
import { CREDIT_USAGE_MUTATIONS } from '@/store/mutationConstants';
import { CreditsApi, CreditUsage } from '@/types/credits';

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
                let credits = await api.get();

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
