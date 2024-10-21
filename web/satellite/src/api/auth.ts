// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorBadRequest } from '@/api/errors/ErrorBadRequest';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { ErrorTooManyRequests } from '@/api/errors/ErrorTooManyRequests';
import {
    AccountDeletionData,
    AccountSetupData,
    FreezeStatus,
    Session,
    SessionsCursor,
    SessionsPage,
    SetUserSettingsData,
    TokenInfo,
    UpdatedUser,
    User,
    UsersApi,
    UserSettings,
} from '@/types/users';
import { HttpClient } from '@/utils/httpClient';
import { ErrorTokenExpired } from '@/api/errors/ErrorTokenExpired';
import { APIError } from '@/utils/error';
import { ErrorTooManyAttempts } from '@/api/errors/ErrorTooManyAttempts';
import { ChangeEmailStep, DeleteAccountStep } from '@/types/accountActions';

/**
 * AuthHttpApi is a console Auth API.
 * Exposes all auth-related functionality
 */
export class AuthHttpApi implements UsersApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/auth';

    /**
     * Used to resend an registration confirmation email.
     *
     * @param email - email of newly created user
     * @param captchaResponse - captcha response token
     * @returns requestID to be used for code activation
     * @throws Error
     */
    public async resendEmail(email: string, captchaResponse: string): Promise<string> {
        const path = `${this.ROOT_PATH}/resend-email`;

        const body = {
            email,
            captchaResponse,
        };

        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return response.headers.get('x-request-id') ?? '';
        }

        const result = await response.json();
        const errMsg = result.error || 'Failed to send email';
        switch (response.status) {
        case 429:
            throw new ErrorTooManyRequests(errMsg);
        default:
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to get authentication token.
     *
     * @param email - email of the user
     * @param password - password of the user
     * @param captchaResponse - captcha response token
     * @param mfaPasscode - MFA passcode
     * @param mfaRecoveryCode - MFA recovery code
     * @param rememberForOneWeek - flag to remember user
     * @throws Error
     */
    public async token(email: string, password: string, captchaResponse: string, mfaPasscode: string, mfaRecoveryCode: string, rememberForOneWeek = false): Promise<TokenInfo> {
        const path = `${this.ROOT_PATH}/token`;
        const body = {
            email,
            password,
            captchaResponse,
            mfaPasscode: mfaPasscode || null,
            mfaRecoveryCode: mfaRecoveryCode || null,
            rememberForOneWeek,
        };

        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            const result = await response.json();
            if (result.error) {
                throw new ErrorMFARequired();
            }

            return new TokenInfo(result.token, new Date(result.expiresAt));
        }

        const result = await response.json();
        const errMsg = result.error || 'Failed to receive authentication token';
        switch (response.status) {
        case 400:
            throw new ErrorBadRequest(errMsg);
        case 429:
            throw new ErrorTooManyRequests(errMsg);
        default:
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to verify signup code.
     * @param email
     * @param code - the code to verify
     * @param signupId - the request ID of the signup request or resend email request
     */
    public async verifySignupCode(email: string, code: string, signupId: string): Promise<TokenInfo> {
        const path = `${this.ROOT_PATH}/code-activation`;
        const body = {
            email,
            code,
            signupId,
        };

        const response = await this.http.patch(path, JSON.stringify(body));
        const result = await response.json();
        if (response.ok) {
            return new TokenInfo(result.token, new Date(result.expiresAt));
        }

        const errMsg = result.error || 'Failed to activate account';
        switch (response.status) {
        case 400:
            throw new ErrorBadRequest(errMsg);
        case 429:
            throw new ErrorTooManyRequests(errMsg);
        default:
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to logout user and delete auth cookie.
     *
     * @throws Error
     */
    public async logout(): Promise<void> {
        const path = `${this.ROOT_PATH}/logout`;
        const response = await this.http.post(path, null);

        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not logout. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });
    }

    public async checkSSO(email: string): Promise<string> {
        const params = new URLSearchParams({ email: email });
        const path = `/sso-url?${params.toString()}`;
        const response = await this.http.get(path);

        if (response.ok) {
            return await response.text();
        }
        throw new APIError({
            status: response.status,
            message: 'Can not check SSO status. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to change user email requests.
     *
     * @throws Error
     */
    public async changeEmail(step: ChangeEmailStep, data: string): Promise<void> {
        const path = `${this.ROOT_PATH}/change-email`;

        const body = JSON.stringify({
            step,
            data,
        });

        const response = await this.http.post(path, body);
        if (response.ok) {
            return;
        }

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not change email. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to mark user's account for deletion.
     *
     * @throws Error
     */
    public async deleteAccount(step: DeleteAccountStep, data: string): Promise<AccountDeletionData | null> {
        const path = `${this.ROOT_PATH}/account`;

        const body = JSON.stringify({
            step,
            data,
        });

        const response = await this.http.delete(path, body);

        if (response.ok) {
            return null;
        }

        const result = await response.json();

        if (response.status === 409) {
            return new AccountDeletionData(
                result.ownedProjects,
                result.buckets,
                result.apiKeys,
                result.unpaidInvoices,
                result.amountOwed,
                result.currentUsage,
                result.invoicingIncomplete,
                result.success,
            );
        }

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not delete account. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });

        return null;
    }

    /**
     * Used to restore password.
     *
     * @param email - email of the user
     * @param captchaResponse - captcha response token
     * @throws Error
     */
    public async forgotPassword(email: string, captchaResponse: string): Promise<void> {
        const path = `${this.ROOT_PATH}/forgot-password`;
        const body = {
            email,
            captchaResponse,
        };
        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return;
        }

        const result = await response.json();
        const errMsg = result.error || 'Failed to send password reset link';
        switch (response.status) {
        case 429:
            throw new ErrorTooManyRequests(errMsg);
        default:
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to update user full and short name.
     *
     * @param userInfo - full name and short name of the user
     * @throws Error
     */
    public async update(userInfo: UpdatedUser): Promise<void> {
        const path = `${this.ROOT_PATH}/account`;
        const body = {
            fullName: userInfo.fullName,
            shortName: userInfo.shortName,
        };
        const response = await this.http.patch(path, JSON.stringify(body));
        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not update user data',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to update user details after signup.
     *
     * @param userInfo - the information to be added to account.
     * @throws Error
     */
    public async setupAccount(userInfo: AccountSetupData): Promise<void> {
        const path = `${this.ROOT_PATH}/account/setup`;
        const response = await this.http.patch(path, JSON.stringify(userInfo));
        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not setup account',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to get user data.
     *
     * @throws Error
     */
    public async get(): Promise<User> {
        const path = `${this.ROOT_PATH}/account`;
        const response = await this.http.get(path);
        if (response.ok) {
            const userResponse = await response.json();

            return new User(
                userResponse.id,
                userResponse.fullName,
                userResponse.shortName,
                userResponse.email,
                userResponse.partner,
                userResponse.password,
                userResponse.projectLimit,
                userResponse.projectStorageLimit,
                userResponse.projectBandwidthLimit,
                userResponse.projectSegmentLimit,
                userResponse.paidTier,
                userResponse.isMFAEnabled,
                userResponse.isProfessional,
                userResponse.position,
                userResponse.companyName,
                userResponse.employeeCount,
                userResponse.haveSalesContact,
                userResponse.mfaRecoveryCodeCount,
                userResponse.createdAt,
                userResponse.pendingVerification,
                userResponse.trialExpiration ? new Date(userResponse.trialExpiration) : null,
                userResponse.hasVarPartner,
            );
        }

        throw new APIError({
            status: response.status,
            message: 'Can not get user data',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to change password.
     *
     * @param password - old password of the user
     * @param newPassword - new password of the user
     * @throws Error
     */
    public async changePassword(password: string, newPassword: string): Promise<void> {
        const path = `${this.ROOT_PATH}/account/change-password`;
        const body = {
            password: password,
            newPassword: newPassword,
        };
        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error,
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to delete account.
     *
     * @param password - password of the user
     * @throws Error
     */
    public async delete(password: string): Promise<void> {
        const path = `${this.ROOT_PATH}/account/delete`;
        const body = {
            password: password,
        };
        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not delete user',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Fetches user frozen status.
     *
     * @throws Error
     */
    public async getFrozenStatus(): Promise<FreezeStatus> {
        const path = `${this.ROOT_PATH}/account/freezestatus`;
        const response = await this.http.get(path);
        if (response.ok) {
            const responseData = await response.json();

            return new FreezeStatus(
                responseData.frozen,
                responseData.warned,
                responseData.trialExpiredFrozen,
                responseData.trialExpirationGracePeriod,
            );
        }

        throw new APIError({
            status: response.status,
            message: 'Can not get user frozen status',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Fetches user's settings.
     *
     * @throws Error
     */
    public async getUserSettings(): Promise<UserSettings> {
        const path = `${this.ROOT_PATH}/account/settings`;
        const response = await this.http.get(path);
        if (response.ok) {
            const responseData = await response.json();

            return new UserSettings(
                responseData.sessionDuration,
                responseData.onboardingStart,
                responseData.onboardingEnd,
                responseData.passphrasePrompt,
                responseData.onboardingStep,
                responseData.noticeDismissal,
            );
        }

        throw new APIError({
            status: response.status,
            message: 'Can not get user settings',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Changes user's settings.
     *
     * @param data
     * @returns UserSettings
     * @throws Error
     */
    public async updateSettings(data: SetUserSettingsData): Promise<UserSettings> {
        const path = `${this.ROOT_PATH}/account/settings`;
        const response = await this.http.patch(path, JSON.stringify(data));
        if (response.ok) {
            const responseData = await response.json();

            return new UserSettings(
                responseData.sessionDuration,
                responseData.onboardingStart,
                responseData.onboardingEnd,
                responseData.passphrasePrompt,
                responseData.onboardingStep,
                responseData.noticeDismissal,
            );
        }

        throw new APIError({
            status: response.status,
            message: 'Can not update user settings',
            requestID: response.headers.get('x-request-id'),
        });
    }

    // TODO: remove secret after Vanguard release
    /**
     * Used to register account.
     *
     * @param user - stores user information
     * @param secret - registration token used in Vanguard release
     * @param captchaResponse - captcha response
     * @returns requestID to be used for code activation.
     * @throws Error
     */
    public async register(user: Partial<User & { storageNeeds: string, isMinimal: boolean }>, secret: string, captchaResponse: string): Promise<string> {
        const path = `${this.ROOT_PATH}/register`;
        const body = {
            secret: secret,
            password: user.password,
            fullName: user.fullName,
            shortName: user.shortName,
            email: user.email,
            partner: user.partner || '',
            isProfessional: user.isProfessional,
            position: user.position,
            companyName: user.companyName,
            storageNeeds: user.storageNeeds || '',
            employeeCount: user.employeeCount,
            haveSalesContact: user.haveSalesContact,
            captchaResponse: captchaResponse,
            signupPromoCode: user.signupPromoCode,
            isMinimal: user.isMinimal,
        };

        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return response.headers.get('x-request-id') ?? '';
        }
        const result = await response.json();
        const errMsg = result.error || 'Cannot register user';
        switch (response.status) {
        case 400:
            throw new ErrorBadRequest(errMsg);
        case 429:
            throw new ErrorTooManyRequests(errMsg);
        default:
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to enable user's MFA.
     *
     * @throws Error
     */
    public async generateUserMFASecret(): Promise<string> {
        const path = `${this.ROOT_PATH}/mfa/generate-secret-key`;
        const response = await this.http.post(path, null);

        if (response.ok) {
            return await response.json();
        }

        throw new APIError({
            status: response.status,
            message: 'Can not generate MFA secret. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to enable user's MFA.
     *
     * @throws Error
     */
    public async enableUserMFA(passcode: string): Promise<string[]> {
        const path = `${this.ROOT_PATH}/mfa/enable`;
        const body = {
            passcode: passcode,
        };

        const response = await this.http.post(path, JSON.stringify(body));

        if (response.ok) {
            return await response.json();
        }

        throw new APIError({
            status: response.status,
            message: 'Can not enable MFA. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to disable user's MFA.
     *
     * @throws Error
     */
    public async disableUserMFA(passcode: string, recoveryCode: string): Promise<void> {
        const path = `${this.ROOT_PATH}/mfa/disable`;
        const body = {
            passcode: passcode || null,
            recoveryCode: recoveryCode || null,
        };

        const response = await this.http.post(path, JSON.stringify(body));

        if (response.ok) {
            return;
        }

        const result = await response.json();
        const errMsg = result.error || 'Cannot disable MFA. Please try again later';
        throw new APIError({
            status: response.status,
            message: errMsg,
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Generate user's MFA recovery codes requiring a code.
     *
     * @throws Error
     */
    public async regenerateUserMFARecoveryCodes(passcode?: string, recoveryCode?: string): Promise<string[]> {
        if (!passcode && !recoveryCode) {
            throw new Error('Either passcode or recovery code should be provided');
        }
        const path = `${this.ROOT_PATH}/mfa/regenerate-recovery-codes`;
        const body = {
            passcode: passcode || null,
            recoveryCode: recoveryCode || null,
        };

        const response = await this.http.post(path, JSON.stringify(body));

        if (response.ok) {
            return await response.json();
        }

        const result = await response.json();
        const errMsg = result.error || 'Cannot regenerate MFA codes. Please try again later';
        throw new APIError({
            status: response.status,
            message: errMsg,
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to reset user's password.
     *
     * @param token - user's password reset token
     * @param password - user's new password
     * @param mfaPasscode - MFA passcode
     * @param mfaRecoveryCode - MFA recovery code
     * @throws Error
     */
    public async resetPassword(token: string, password: string, mfaPasscode: string, mfaRecoveryCode: string): Promise<void> {
        const path = `${this.ROOT_PATH}/reset-password`;

        const body = {
            token: token,
            password: password,
            mfaPasscode: mfaPasscode || null,
            mfaRecoveryCode: mfaRecoveryCode || null,
        };

        const response = await this.http.post(path, JSON.stringify(body));
        const text = await response.text();
        let errMsg = 'Cannot reset password';

        if (text) {
            const result = JSON.parse(text);
            if (result.code === 'too_many_attempts') {
                throw new ErrorTooManyAttempts();
            }
            if (result.code === 'mfa_required') {
                throw new ErrorMFARequired();
            }
            if (result.code === 'token_expired') {
                throw new ErrorTokenExpired();
            }
            if (result.error) {
                errMsg = result.error;
            }
        }

        if (response.ok) {
            return;
        }

        switch (response.status) {
        case 400:
            throw new ErrorBadRequest(errMsg);
        default:
            throw new APIError({
                status: response.status,
                message: errMsg,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to refresh the expiration time of the current session.
     *
     * @returns new expiration timestamp
     * @throws Error
     */
    public async refreshSession(): Promise<Date> {
        const path = `${this.ROOT_PATH}/refresh-session`;
        const response = await this.http.post(path, null);

        if (response.ok) {
            return new Date(await response.json());
        }

        throw new APIError({
            status: response.status,
            message: 'Unable to refresh session.',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to fetch active user sessions.
     *
     * @throws Error
     */
    public async getSessions(cursor: SessionsCursor): Promise<SessionsPage> {
        const path = `${this.ROOT_PATH}/sessions?limit=${cursor.limit}&page=${cursor.page}&order=${cursor.order}&orderDirection=${cursor.orderDirection}`;
        const response = await this.http.get(path);
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Can not get user sessions',
                requestID: response.headers.get('x-request-id'),
            });
        }

        if (!(result && result.sessions)) {
            return new SessionsPage();
        }

        const sessionsPage: SessionsPage = new SessionsPage();

        sessionsPage.sessions = result.sessions.map(session => new Session(
            session.id,
            session.userAgent,
            new Date(session.expiresAt),
            session.isRequesterCurrentSession,
        ));

        sessionsPage.limit = result.limit;
        sessionsPage.order = result.order;
        sessionsPage.orderDirection = result.orderDirection;
        sessionsPage.pageCount = result.pageCount;
        sessionsPage.currentPage = result.currentPage;
        sessionsPage.totalCount = result.totalCount;

        return sessionsPage;
    }

    /**
     * Used to invalidate active user session by ID.
     *
     * @throws Error
     */
    public async invalidateSession(sessionID: string): Promise<void> {
        const path = `${this.ROOT_PATH}/invalidate-session/${sessionID}`;
        const response = await this.http.post(path, null);

        if (!response.ok) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.error || 'Can not invalidate session',
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to request increase for user's project limit.
     *
     * @param limit
     */
    public async requestProjectLimitIncrease(limit: string): Promise<void> {
        const path = `${this.ROOT_PATH}/limit-increase`;
        const response = await this.http.patch(path, limit);

        if (!response.ok) {
            const result = await response.json();
            throw new APIError({
                status: response.status,
                message: result.error,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }
}
