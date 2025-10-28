// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import {
    ACCOUNT_SETUP_STEPS,
    AccountDeletionData,
    DisableMFARequest,
    OnboardingStep,
    SessionsCursor,
    SessionsOrderBy,
    SessionsPage,
    SetUserSettingsData,
    UpdatedUser,
    User,
    UsersApi,
    UserSettings,
} from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { useConfigStore } from '@/store/modules/configStore';
import { ChangeEmailStep, DeleteAccountStep } from '@/types/accountActions';
import { SortDirection } from '@/types/common';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { AuthManagementHttpApiV1 } from '@/api/private.gen';

export class UsersState {
    public user: User = new User();
    public settings: UserSettings = new UserSettings();
    public userMFASecret = '';
    public userMFARecoveryCodes: string[] = [];
    public sessionsCursor: SessionsCursor = new SessionsCursor();
    public sessionsPage: SessionsPage = new SessionsPage();
    public badPasswords: Set<string> = new Set();
}

export const useUsersStore = defineStore('users', () => {
    const state = reactive<UsersState>(new UsersState());

    const configStore = useConfigStore();
    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);
    const useGeneratedAPI = computed<boolean>(() => configStore.state.config.useGeneratedPrivateAPI);

    const userName = computed(() => {
        return state.user.getFullName();
    });

    const noticeDismissal = computed(() => state.settings.noticeDismissal);

    const api: UsersApi = new AuthHttpApi();
    const generatedAPI = new AuthManagementHttpApiV1();

    async function updateUser(userInfo: UpdatedUser): Promise<void> {
        await api.update(userInfo, csrfToken.value);

        state.user.fullName = userInfo.fullName;
        state.user.shortName = userInfo.shortName;
    }

    async function changeEmail(step: ChangeEmailStep, data: string): Promise<void> {
        await api.changeEmail(step, data, csrfToken.value);
    }

    async function getBadPasswords(): Promise<void> {
        state.badPasswords = await api.getBadPasswords();
    }

    async function deleteAccount(step: DeleteAccountStep, data: string): Promise<AccountDeletionData | null> {
        return await api.deleteAccount(step, data, csrfToken.value);
    }

    async function getSessions(pageNumber: number, limit = DEFAULT_PAGE_LIMIT): Promise<SessionsPage> {
        state.sessionsCursor.page = pageNumber;
        state.sessionsCursor.limit = limit;

        const sessionsPage: SessionsPage = await api.getSessions(state.sessionsCursor);

        state.sessionsPage = sessionsPage;

        return sessionsPage;
    }

    async function invalidateSession(sessionID: string): Promise<void> {
        await api.invalidateSession(sessionID, csrfToken.value);
    }

    function setSessionsSortingBy(order: SessionsOrderBy): void {
        state.sessionsCursor.order = order;
    }

    function setSessionsSortingDirection(direction: SortDirection): void {
        state.sessionsCursor.orderDirection = direction;
    }

    async function getUser(): Promise<void> {
        const configStore = useConfigStore();

        let user: User;
        if (useGeneratedAPI.value) {
            user = User.fromUserAccount(await generatedAPI.getUserAccount());
        } else {
            user = await api.get();
        }
        user.projectLimit ||= configStore.state.config.defaultProjectLimit;

        state.user = user;
    }

    function getShouldPromptPassphrase(isProjectOwner: boolean): boolean {
        const settings = state.settings;
        const step = settings.onboardingStep as OnboardingStep || OnboardingStep.AccountInfo;
        if (!settings.passphrasePrompt) {
            return false;
        }
        if (!settings.onboardingEnd && ACCOUNT_SETUP_STEPS.includes(step)) {
            return false;
        }
        if (isProjectOwner) {
            return settings.onboardingEnd || step !== OnboardingStep.EncryptionPassphrase;
        }
        return true;
    }

    async function disableUserMFA(request: DisableMFARequest): Promise<void> {
        await api.disableUserMFA(request.passcode, request.recoveryCode, csrfToken.value);
    }

    async function enableUserMFA(passcode: string): Promise<void> {
        const recoveryCodes = await api.enableUserMFA(passcode, csrfToken.value);

        state.userMFARecoveryCodes = recoveryCodes;
        state.user.mfaRecoveryCodeCount = recoveryCodes.length;
    }

    async function generateUserMFASecret(): Promise<void> {
        state.userMFASecret = await api.generateUserMFASecret(csrfToken.value);
    }

    async function regenerateUserMFARecoveryCodes(code: { recoveryCode?: string, passcode?: string }): Promise<void> {
        const codes = await api.regenerateUserMFARecoveryCodes(csrfToken.value, code.passcode, code.recoveryCode);

        state.userMFARecoveryCodes = codes;
        state.user.mfaRecoveryCodeCount = codes.length;
    }

    async function getSettings(): Promise<UserSettings> {
        const settings = await api.getUserSettings();

        state.settings = settings;

        return settings;
    }

    async function updateSettings(update: SetUserSettingsData): Promise<void> {
        state.settings = await api.updateSettings(update, csrfToken.value);
    }

    async function requestProjectLimitIncrease(limit: string): Promise<void> {
        await api.requestProjectLimitIncrease(limit);
    }

    // Does nothing. It is called on login screen, and we just subscribe to this action in dashboard wrappers.
    function login(): void { }

    function clear() {
        state.user = new User();
        state.settings = new UserSettings();
        state.userMFASecret = '';
        state.userMFARecoveryCodes = [];
    }

    return {
        state,
        userName,
        noticeDismissal,
        invalidateSession,
        getSessions,
        setSessionsSortingBy,
        setSessionsSortingDirection,
        updateUser,
        changeEmail,
        deleteAccount,
        getUser,
        disableUserMFA,
        enableUserMFA,
        generateUserMFASecret,
        regenerateUserMFARecoveryCodes,
        getShouldPromptPassphrase,
        clear,
        login,
        updateSettings,
        getSettings,
        requestProjectLimitIncrease,
        getBadPasswords,
    };
});
