// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { ReferralApi, ReferralLink } from '@/types/referral';

export const REFERRAL_ACTIONS = {
    GET_TOKENS: 'getReferralTokens',
};

export const REFERRAL_MUTATIONS = {
    SET_TOKENS: 'setReferralTokens',
};

const {
    GET_TOKENS,
} = REFERRAL_ACTIONS;

const {
    SET_TOKENS,
} = REFERRAL_MUTATIONS;

export class ReferralState {
    public referralTokens: string[] = [];
}

/**
 * creates referral module with all dependencies
 *
 * @param api - referral api
 */
export function makeReferralModule(api: ReferralApi): StoreModule<ReferralState> {
    return {
        state: new ReferralState(),
        mutations: {
            [SET_TOKENS](state: ReferralState, referralTokens: string[]): void {
                state.referralTokens = referralTokens;
            },
        },
        actions: {
            [GET_TOKENS]: async function ({commit}: any): Promise<string[]> {
                const referralTokens = await api.getTokens();

                commit(SET_TOKENS, referralTokens);

                return referralTokens;
            },
        },
        getters: {
            referralLinks: (state: ReferralState): ReferralLink[] => {
                return state.referralTokens.map(token => {
                    return new ReferralLink(token);
                });
            },
        },
    };
}
