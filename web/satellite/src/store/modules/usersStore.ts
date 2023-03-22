// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import { DisableMFARequest, UpdatedUser, User, UsersApi } from '@/types/users';
import { MetaUtils } from '@/utils/meta';
import { AuthHttpApi } from '@/api/auth';

export class UsersState {
    public user: User = new User();
    public userMFASecret = '';
    public userMFARecoveryCodes: string[] = [];
}

export const useUsersStore = defineStore('users', () => {
    const state = reactive<UsersState>(new UsersState());

    const userName = computed(() => {
        return state.user.getFullName();
    });

    const api: UsersApi = new AuthHttpApi();

    async function updateUserInfo(userInfo: UpdatedUser): Promise<void> {
        await api.update(userInfo);

        state.user.fullName = userInfo.fullName;
        state.user.shortName = userInfo.shortName;
    }

    async function fetchUserInfo(): Promise<void> {
        const user = await api.get();

        state.user = user;

        if (user.projectLimit === 0) {
            const limitFromConfig = MetaUtils.getMetaContent('default-project-limit');

            state.user.projectLimit = parseInt(limitFromConfig);

            return;
        }

        state.user.projectLimit = user.projectLimit;
    }

    async function fetchFrozenStatus(): Promise<void> {
        state.user.isFrozen = await api.getFrozenStatus();
    }

    async function disableUserMFA(request: DisableMFARequest): Promise<void> {
        await api.disableUserMFA(request.passcode, request.recoveryCode);
    }

    async function enableUserMFA(passcode: string): Promise<void> {
        await api.enableUserMFA(passcode);
    }

    async function generateUserMFASecret(): Promise<void> {
        state.userMFASecret = await api.generateUserMFASecret();
    }

    async function generateUserMFARecoveryCodes(): Promise<void> {
        const codes = await api.generateUserMFARecoveryCodes();

        state.userMFARecoveryCodes = codes;
        state.user.mfaRecoveryCodeCount = codes.length;
    }

    function clearUserInfo() {
        state.user = new User();
        state.user.projectLimit = 1;
    }

    return {
        usersState: state,
        userName,
        updateUserInfo,
        fetchUserInfo,
        disableUserMFA,
        enableUserMFA,
        generateUserMFASecret,
        generateUserMFARecoveryCodes,
        clearUserInfo,
        fetchFrozenStatus,
    };
});
