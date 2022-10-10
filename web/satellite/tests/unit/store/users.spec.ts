// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

import { AuthHttpApi } from '@/api/auth';
import { makeUsersModule, USER_ACTIONS, USER_MUTATIONS } from '@/store/modules/users';
import { UpdatedUser, User } from '@/types/users';

const Vue = createLocalVue();
const authApi = new AuthHttpApi();
const usersModule = makeUsersModule(authApi);
const { UPDATE, GET, CLEAR } = USER_ACTIONS;

Vue.use(Vuex);

const store = new Vuex.Store<typeof usersModule.state>(usersModule);

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });
    it('Set user', () => {
        const user = new User('1', 'fullName', 'shortName', 'user@example.com');

        store.commit(USER_MUTATIONS.SET_USER, user);

        expect(store.state.user.id).toBe(user.id);
        expect(store.state.user.email).toBe(user.email);
        expect(store.state.user.fullName).toBe(user.fullName);
        expect(store.state.user.shortName).toBe(user.shortName);
    });

    it('clear user', () => {
        store.commit(USER_MUTATIONS.CLEAR);

        expect(store.state.user.id).toBe('');
        expect(store.state.user.email).toBe('');
        expect(store.state.user.fullName).toBe('');
        expect(store.state.user.shortName).toBe('');
    });

    it('Update user', () => {
        const user = new UpdatedUser('fullName', 'shortName');

        store.commit(USER_MUTATIONS.UPDATE_USER, user);

        expect(store.state.user.fullName).toBe(user.fullName);
        expect(store.state.user.shortName).toBe(user.shortName);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });
    it('success update account', async () => {
        jest.spyOn(authApi, 'update').mockReturnValue(
            Promise.resolve(),
        );

        const user = new UpdatedUser('fullName1', 'shortName2');

        await store.dispatch(UPDATE, user);

        expect(store.state.user.fullName).toBe('fullName1');
        expect(store.state.user.shortName).toBe('shortName2');
    });

    it('update throws an error when api call fails', async () => {
        jest.spyOn(authApi, 'update').mockImplementation(() => { throw new Error(); });
        const newUser = new UpdatedUser('', '');
        const oldUser = store.getters.user;

        try {
            await store.dispatch(UPDATE, newUser);
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.user.fullName).toBe(oldUser.fullName);
            expect(store.state.user.shortName).toBe(oldUser.shortName);
        }
    });

    it('clears state', async () => {
        await store.dispatch(CLEAR);

        expect(store.state.user.fullName).toBe('');
        expect(store.state.user.shortName).toBe('');
        expect(store.state.user.email).toBe('');
        expect(store.state.user.partnerId).toBe('');
        expect(store.state.user.id).toBe('');
    });

    it('success get user', async () => {
        const user = new User('2', 'newFullName', 'newShortName', 'user2@example.com');

        jest.spyOn(authApi, 'get').mockReturnValue(
            Promise.resolve(user),
        );

        await store.dispatch(GET);

        expect(store.state.user.id).toBe(user.id);
        expect(store.state.user.shortName).toBe(user.shortName);
        expect(store.state.user.fullName).toBe(user.fullName);
        expect(store.state.user.email).toBe(user.email);
    });

    it('get throws an error when api call fails', async () => {
        const user = store.getters.user;
        jest.spyOn(authApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(GET);
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.user.fullName).toBe(user.fullName);
            expect(store.state.user.shortName).toBe(user.shortName);
        }
    });
});

describe('getters', () => {
    it('user model', function () {
        const retrievedUser = store.getters.user;

        expect(retrievedUser.id).toBe(store.state.user.id);
        expect(retrievedUser.fullName).toBe(store.state.user.fullName);
        expect(retrievedUser.shortName).toBe(store.state.user.shortName);
        expect(retrievedUser.email).toBe(store.state.user.email);
    });

    it('user name', function () {
        const retrievedUserName = store.getters.userName;

        expect(retrievedUserName).toBe(store.state.user.getFullName());
    });
});
