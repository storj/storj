// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { REWARD_MUTATIONS } from '../mutationConstants';
import {
    getCurrentReward,
} from '@/api/rewards';

const initState = {
    reward: {
        id: undefined,
        awardCredit: 0,
        inviteeCredit: 0,
        redeemableCap: 0,
        awardCreditDurationDays: 0,
        inviteeCreditDurationDays: 0,
        expiresAt: Date.now(),
        status: 0,
        type: 0,
    }
};

export const usersModule = {
    state: initState,
    mutations: {
        [REWARD_MUTATIONS.SET_REWARD_INFO](state: any, reward: Reward): void {
            state.reward = reward;
        },

        [USER_MUTATIONS.REVERT_TO_DEFAULT_REWARD_INFO](state: any): void {
            state = initState;
        },
    },

    actions: {
        getReward: async function ({commit}: any): Promise<RequestResponse<Reward>> {
            let response = await getCurrentReward();

            if (response.isSuccess) {
                commit(REWARD_MUTATIONS.SET_REWARD_INFO, response.data);
            }

            return response;
        },
        clearUser: function({commit}: any) {
            commit(REWARD_MUTATIONS.CLEAR);
        },
    },

    getters: {
        user: (state: any) => {
            return state.user;
        },
        userName: (state: any) => state.user.shortName == '' ? state.user.fullName : state.user.shortName
    },
};
