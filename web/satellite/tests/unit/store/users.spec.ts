// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { AuthHttpApi } from '@/api/auth';
import { makeUsersModule, USER_ACTIONS, USER_MUTATIONS } from '@/store/modules/users';
import { UpdatedUser, User } from '@/types/users';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();
const authApi = new AuthHttpApi();
const usersModule = makeUsersModule(authApi);
const { UPDATE, GET, CLEAR } = USER_ACTIONS;

Vue.use(Vuex);

const store = new Vuex.Store(usersModule);

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });
    it('Set user', () => {
        const user = new User('1', 'fullName', 'shortName', 'example@email.com');

        store.commit(USER_MUTATIONS.SET_USER, user);

        expect(store.state.id).toBe(user.id);
        expect(store.state.email).toBe(user.email);
        expect(store.state.fullName).toBe(user.fullName);
        expect(store.state.shortName).toBe(user.shortName);
    });

    it('clear user', () => {
        store.commit(USER_MUTATIONS.CLEAR);

        expect(store.state.id).toBe('');
        expect(store.state.email).toBe('');
        expect(store.state.fullName).toBe('');
        expect(store.state.shortName).toBe('');
    });

    it('Update user', () => {
        const user = new UpdatedUser('fullName', 'shortName');

        store.commit(USER_MUTATIONS.UPDATE_USER, user);

        expect(store.state.fullName).toBe(user.fullName);
        expect(store.state.shortName).toBe(user.shortName);
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

        expect(store.state.fullName).toBe('fullName1');
        expect(store.state.shortName).toBe('shortName2');
    });

    it('update throws an error when api call fails', async () => {
        jest.spyOn(authApi, 'update').mockImplementation(() => { throw new Error(); });
        const newUser = new UpdatedUser('', '');
        const oldUser = store.getters.user;

        try {
            await store.dispatch(UPDATE, newUser);
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.fullName).toBe(oldUser.fullName);
            expect(store.state.shortName).toBe(oldUser.shortName);
        }
    });

    it('clears state', async () => {
        await store.dispatch(CLEAR);

        expect(store.state.fullName).toBe('');
        expect(store.state.shortName).toBe('');
        expect(store.state.email).toBe('');
        expect(store.state.partnerId).toBe('');
        expect(store.state.id).toBe('');
    });

    it('success get user', async () => {
        const user = new User('2', 'newFullName', 'newShortName', 'example2@email.com');

        jest.spyOn(authApi, 'get').mockReturnValue(
            Promise.resolve(user),
        );

        await store.dispatch(GET);

        expect(store.state.id).toBe(user.id);
        expect(store.state.shortName).toBe(user.shortName);
        expect(store.state.fullName).toBe(user.fullName);
        expect(store.state.email).toBe(user.email);
    });

    it('get throws an error when api call fails', async () => {
        const user = store.getters.user;
        jest.spyOn(authApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(GET);
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.fullName).toBe(user.fullName);
            expect(store.state.shortName).toBe(user.shortName);
        }
    });
});

describe('getters', () => {
    it('user model', function () {
        const retrievedUser = store.getters.user;

        expect(retrievedUser.id).toBe(store.state.id);
        expect(retrievedUser.fullName).toBe(store.state.fullName);
        expect(retrievedUser.shortName).toBe(store.state.shortName);
        expect(retrievedUser.email).toBe(store.state.email);
    });

    it('user name', function () {
        const retrievedUserName = store.getters.userName;

        expect(retrievedUserName).toBe(store.state.getFullName());
    });
});
