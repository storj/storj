// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { ReferralApi } from '@/types/referral';

export const REFERRAL_ACTIONS = {
    GET_LINKS: 'getReferralLinks',
};

export const REFERRAL_MUTATIONS = {
    SET_LINKS: 'setReferralLinks',
};

const {
    GET_LINKS,
} = REFERRAL_ACTIONS;

const {
    SET_LINKS,
} = REFERRAL_MUTATIONS;

export class ReferralState {
    public referralLinks = [];
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
            [SET_LINKS](state: ReferralState, referralLinks): void {
                state.referralLinks = referralLinks;
            },
        },
        actions: {
            [GET_LINKS]: async function ({commit}: any): Promise<any> {
                const referralLinks = await api.getLinks();

                commit(GET_LINKS, referralLinks);

                return referralLinks;
            },
        },
    };
}
