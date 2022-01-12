// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorBadRequest } from '@/api/errors/ErrorBadRequest';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { ErrorTooManyRequests } from '@/api/errors/ErrorTooManyRequests';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { UpdatedUser, User, UsersApi } from '@/types/users';
import { HttpClient } from '@/utils/httpClient';

/**
 * AuthHttpApi is a console Auth API.
 * Exposes all auth-related functionality
 */
export class AuthHttpApi implements UsersApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/auth';
    private readonly rateLimitErrMsg = 'You\'ve exceeded limit of attempts, try again in 5 minutes';

    /**
     * Used to resend an registration confirmation email.
     *
     * @param email - email of newly created user
     * @throws Error
     */
    public async resendEmail(email: string): Promise<void> {
        const path = `${this.ROOT_PATH}/resend-email/${email}`;
        const response = await this.http.post(path, email);
        if (response.ok) {
            return;
        }

        if (response.status == 429) {
            throw new ErrorTooManyRequests(this.rateLimitErrMsg);
        }

        throw new Error('Failed to send email');
    }

    /**
     * Used to get authentication token.
     *
     * @param email - email of the user
     * @param password - password of the user
     * @param mfaPasscode - MFA passcode
     * @param mfaRecoveryCode - MFA recovery code
     * @throws Error
     */
    public async token(email: string, password: string, mfaPasscode: string, mfaRecoveryCode: string): Promise<string> {
        const path = `${this.ROOT_PATH}/token`;
        const body = {
            email,
            password,
            mfaPasscode: mfaPasscode || null,
            mfaRecoveryCode: mfaRecoveryCode || null,
        };

        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            const result = await response.json();
            if (typeof result !== 'string') {
                throw new ErrorMFARequired();
            }

            return result;
        }

        const result = await response.json();
        const errMsg = result.error || 'Failed to receive authentication token';
        switch (response.status) {
        case 401:
            throw new ErrorUnauthorized(errMsg);
        case 429:
            throw new ErrorTooManyRequests(this.rateLimitErrMsg);
        default:
            throw new Error(errMsg);
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

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('Can not logout. Please try again later');
    }

    /**
     * Used to restore password.
     *
     * @param email - email of the user
     * @throws Error
     */
    public async forgotPassword(email: string): Promise<void> {
        const path = `${this.ROOT_PATH}/forgot-password/${email}`;
        const response = await this.http.post(path, email);
        if (response.ok) {
            return;
        }

        const result = await response.json();
        const errMsg = result.error || 'Failed to send password reset link';
        switch (response.status) {
        case 404:
            throw new ErrorUnauthorized(errMsg);
        case 429:
            throw new ErrorTooManyRequests(this.rateLimitErrMsg);
        default:
            throw new Error(errMsg);
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

        throw new Error('can not update user data');
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
                userResponse.partnerId,
                userResponse.password,
                userResponse.projectLimit,
                userResponse.paidTier,
                userResponse.isMFAEnabled,
                userResponse.isProfessional,
                userResponse.position,
                userResponse.companyName,
                userResponse.employeeCount,
                userResponse.haveSalesContact,
                userResponse.mfaRecoveryCodeCount,
            );
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not get user data');
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

        switch (response.status) {
        case 401: {
            throw new Error('old password is incorrect, please try again');
        }
        default: {
            throw new Error('can not change password');
        }
        }
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

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not delete user');
    }

    // TODO: remove secret after Vanguard release
    /**
     * Used to register account.
     *
     * @param user - stores user information
     * @param secret - registration token used in Vanguard release
     * @param recaptchaResponse - recaptcha response
     * @returns id of created user
     * @throws Error
     */
    public async register(user: {fullName: string; shortName: string; email: string; partner: string; partnerId: string; password: string; isProfessional: boolean; position: string; companyName: string; employeeCount: string; haveSalesContact: boolean, signupPromoCode: string }, secret: string, recaptchaResponse: string): Promise<string> {
        const path = `${this.ROOT_PATH}/register`;
        const body = {
            secret: secret,
            password: user.password,
            fullName: user.fullName,
            shortName: user.shortName,
            email: user.email,
            partner: user.partner ? user.partner : '',
            partnerId: user.partnerId ? user.partnerId : '',
            isProfessional: user.isProfessional,
            position: user.position,
            companyName: user.companyName,
            employeeCount: user.employeeCount,
            haveSalesContact: user.haveSalesContact,
            recaptchaResponse: recaptchaResponse,
            signupPromoCode: user.signupPromoCode,
        };
        const response = await this.http.post(path, JSON.stringify(body));
        const result = await response.json();
        if (!response.ok) {
            const errMsg = result.error || 'Cannot register user';
            switch (response.status) {
            case 400:
                throw new ErrorBadRequest(errMsg);
            case 401:
                throw new ErrorUnauthorized(errMsg);
            case 429:
                throw new ErrorTooManyRequests(this.rateLimitErrMsg);
            default:
                throw new Error(errMsg);
            }
        }
        return result;
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

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('Can not generate MFA secret. Please try again later');
    }

    /**
     * Used to enable user's MFA.
     *
     * @throws Error
     */
    public async enableUserMFA(passcode: string): Promise<void> {
        const path = `${this.ROOT_PATH}/mfa/enable`;
        const body = {
            passcode: passcode,
        };

        const response = await this.http.post(path, JSON.stringify(body));

        if (response.ok) {
            return;
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('Can not enable MFA. Please try again later');
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
        if (!response.ok) {
            const errMsg = result.error || 'Cannot disable MFA. Please try again later';
            switch (response.status) {
            case 401:
                throw new ErrorUnauthorized(errMsg);
            default:
                throw new Error(errMsg);
            }
        }
    }

    /**
     * Used to generate user's MFA recovery codes.
     *
     * @throws Error
     */
    public async generateUserMFARecoveryCodes(): Promise<string[]> {
        const path = `${this.ROOT_PATH}/mfa/generate-recovery-codes`;
        const response = await this.http.post(path, null);

        if (response.ok) {
            return await response.json();
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('Can not generate MFA recovery codes. Please try again later');
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
            if (result.code == "mfa_required") {
                throw new ErrorMFARequired();
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
        case 401:
            throw new ErrorUnauthorized(errMsg);
        default:
            throw new Error(errMsg);
        }
    }
}
