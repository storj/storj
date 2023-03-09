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
    const usersState = reactive<UsersState>({
        user: new User(),
        userMFASecret: '',
        userMFARecoveryCodes: [],
    });

    const userName = computed(() => {
        return usersState.user.getFullName();
    });

    const api: UsersApi = new AuthHttpApi();

    async function updateUserInfo(userInfo: UpdatedUser): Promise<void> {
        await api.update(userInfo);

        usersState.user.fullName = userInfo.fullName;
        usersState.user.shortName = userInfo.shortName;
    }

    async function fetchUserInfo(): Promise<void> {
        const user = await api.get();

        usersState.user = user;

        if (user.projectLimit === 0) {
            const limitFromConfig = MetaUtils.getMetaContent('default-project-limit');

            usersState.user.projectLimit = parseInt(limitFromConfig);

            return;
        }

        usersState.user.projectLimit = user.projectLimit;
    }

    async function fetchFrozenStatus(): Promise<void> {
        usersState.user.isFrozen = await api.getFrozenStatus();
    }

    async function disableUserMFA(request: DisableMFARequest): Promise<void> {
        await api.disableUserMFA(request.passcode, request.recoveryCode);
    }

    async function enableUserMFA(passcode: string): Promise<void> {
        await api.enableUserMFA(passcode);
    }

    async function generateUserMFASecret(): Promise<void> {
        usersState.userMFASecret = await api.generateUserMFASecret();
    }

    async function generateUserMFARecoveryCodes(): Promise<void> {
        const codes = await api.generateUserMFARecoveryCodes();

        usersState.userMFARecoveryCodes = codes;
        usersState.user.mfaRecoveryCodeCount = codes.length;
    }

    function clearUserInfo() {
        usersState.user = new User();
        usersState.user.projectLimit = 1;
    }

    return {
        usersState,
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
