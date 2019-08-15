// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { USER_MUTATIONS } from '../mutationConstants';
import { UpdatedUser, User, UsersApi } from '@/types/users';
import { StoreModule } from '@/store';

const {
    SET_USER,
    UPDATE_USER,
    CLEAR,
} = USER_MUTATIONS;

/**
 * creates users module with all dependencies
 *
 * @param api - users api
 */
export function makeUsersModule(api: UsersApi): StoreModule<User> {
    return {
        state: new User(),

        mutations: {
            setUser(state: User, user: User): void {
                state.id = user.id;
                state.email = user.email;
                state.shortName = user.shortName;
                state.fullName = user.fullName;
                state.partnerId = user.partnerId;
            },

            clearUser(state: User): void {
                state.id = '';
                state.email = '';
                state.shortName = '';
                state.fullName = '';
                state.partnerId = '';
            },

            updateUser(state: User, user: UpdatedUser): void {
                state.fullName = user.fullName;
                state.shortName = user.shortName;
            },
        },

        actions: {
            updateUser: async function ({commit}: any, userInfo: UpdatedUser): Promise<void> {
                await api.update(userInfo);

                commit(UPDATE_USER, userInfo);
            },
            getUser: async function ({commit}: any): Promise<User> {
                let user = await api.get();

                commit(SET_USER, user);

                return user;
            },
            clearUser: function({commit}: any) {
                commit(CLEAR);
            },
        },

        getters: {
            user: (state: User): User => state,
            userName: (state: User): string => state.getFullName(),
        },
    };
}
