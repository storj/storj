// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import {
    FreezeEventType,
    FreezeUserRequest,
    UserAccount,
    UserManagementHttpApiV1,
} from '@/api/client.gen';

class UsersState {
    public userAccount: UserAccount | null = null;
    public freezeTypes: FreezeEventType[] = [];
}

export const useUsersStore = defineStore('users', () => {
    const state = reactive<UsersState>(new UsersState());

    const userApi = new UserManagementHttpApiV1();

    async function getUserByEmail(email: string): Promise<UserAccount> {
        state.userAccount = await userApi.getUserByEmail(email);

        return state.userAccount;
    }

    async function getAccountFreezeTypes(): Promise<void> {
        state.freezeTypes = await userApi.getFreezeEventTypes();
    }

    async function freezeUser(userID: string, eventType: number): Promise<void> {
        const request = new FreezeUserRequest();
        request.type = eventType;
        await userApi.freezeUser(request, userID);
    }

    async function unfreezeUser(userID: string): Promise<void> {
        await userApi.unfreezeUser(userID);
    }

    return {
        state,
        getUserByEmail,
        getAccountFreezeTypes,
        freezeUser,
        unfreezeUser,
    };
});
