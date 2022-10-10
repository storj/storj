// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { DisableMFARequest, UpdatedUser, User, UsersApi } from '@/types/users';
import { MetaUtils } from '@/utils/meta';
import { StoreModule } from '@/types/store';

export const USER_ACTIONS = {
    LOGIN: 'loginUser',
    UPDATE: 'updateUser',
    GET: 'getUser',
    ENABLE_USER_MFA: 'enableUserMFA',
    DISABLE_USER_MFA: 'disableUserMFA',
    GENERATE_USER_MFA_SECRET: 'generateUserMFASecret',
    GENERATE_USER_MFA_RECOVERY_CODES: 'generateUserMFARecoveryCodes',
    CLEAR: 'clearUser',
};

export const USER_MUTATIONS = {
    SET_USER: 'setUser',
    SET_USER_MFA_SECRET: 'setUserMFASecret',
    SET_USER_MFA_RECOVERY_CODES: 'setUserMFARecoveryCodes',
    UPDATE_USER: 'updateUser',
    CLEAR: 'clearUser',
};

export class UsersState {
    public user: User = new User();
    public userMFASecret = '';
    public userMFARecoveryCodes: string[] = [];
}

const {
    GET,
    UPDATE,
    ENABLE_USER_MFA,
    DISABLE_USER_MFA,
    GENERATE_USER_MFA_SECRET,
    GENERATE_USER_MFA_RECOVERY_CODES,
} = USER_ACTIONS;

const {
    SET_USER,
    UPDATE_USER,
    SET_USER_MFA_SECRET,
    SET_USER_MFA_RECOVERY_CODES,
    CLEAR,
} = USER_MUTATIONS;

interface UsersContext {
    state: UsersState
    commit: (string, ...unknown) => void
}

/**
 * creates users module with all dependencies
 *
 * @param api - users api
 */
export function makeUsersModule(api: UsersApi): StoreModule<UsersState, UsersContext> {
    return {
        state: new UsersState(),

        mutations: {
            [SET_USER](state: UsersState, user: User): void {
                state.user = user;

                if (user.projectLimit === 0) {
                    const limitFromConfig = MetaUtils.getMetaContent('default-project-limit');

                    state.user.projectLimit = parseInt(limitFromConfig);

                    return;
                }

                state.user.projectLimit = user.projectLimit;
            },
            [CLEAR](state: UsersState): void {
                state.user = new User();
                state.user.projectLimit = 1;
            },
            [UPDATE_USER](state: UsersState, user: UpdatedUser): void {
                state.user.fullName = user.fullName;
                state.user.shortName = user.shortName;
            },
            [SET_USER_MFA_SECRET](state: UsersState, secret: string): void {
                state.userMFASecret = secret;
            },
            [SET_USER_MFA_RECOVERY_CODES](state: UsersState, codes: string[]): void {
                state.userMFARecoveryCodes = codes;
                state.user.mfaRecoveryCodeCount = codes.length;
            },
        },

        actions: {
            [UPDATE]: async function ({ commit }: UsersContext, userInfo: UpdatedUser): Promise<void> {
                await api.update(userInfo);

                commit(UPDATE_USER, userInfo);
            },
            [GET]: async function ({ commit }: UsersContext): Promise<User> {
                const user = await api.get();

                commit(SET_USER, user);

                return user;
            },
            [DISABLE_USER_MFA]: async function (_, request: DisableMFARequest): Promise<void> {
                await api.disableUserMFA(request.passcode, request.recoveryCode);
            },
            [ENABLE_USER_MFA]: async function (_, passcode: string): Promise<void> {
                await api.enableUserMFA(passcode);
            },
            [GENERATE_USER_MFA_SECRET]: async function ({ commit }: UsersContext): Promise<void> {
                const secret = await api.generateUserMFASecret();

                commit(SET_USER_MFA_SECRET, secret);
            },
            [GENERATE_USER_MFA_RECOVERY_CODES]: async function ({ commit }: UsersContext): Promise<void> {
                const codes = await api.generateUserMFARecoveryCodes();

                commit(SET_USER_MFA_RECOVERY_CODES, codes);
            },
            [CLEAR]: function({ commit }: UsersContext) {
                commit(CLEAR);
            },
        },

        getters: {
            user: (state: UsersState): User => state.user,
            userName: (state: UsersState): string => state.user.getFullName(),
        },
    };
}
