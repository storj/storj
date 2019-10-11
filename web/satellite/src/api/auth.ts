// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseRest } from '@/api/baseRest';
import { User } from '@/types/users';

/**
 * AuthApiGql is a graphql implementation of Auth API.
 * Exposes all auth-related functionality
 */
export class AuthApi extends BaseRest {
    /**
     * Used to resend an registration confirmation email
     *
     * @param userId - id of newly created user
     * @throws Error
     */
    public async resendEmail(userId: string): Promise<void> {
        const path = `/users/${userId}/resend-email`;
        await this.sendRequest('GET', path, null);
    }

    /**
     * Used to get authentication token
     *
     * @param email - email of the user
     * @param password - password of the user
     * @throws Error
     */
    public async token(email: string, password: string): Promise<string> {
        const path = '/users/token';
        const body = {
            email: email,
            password: password,
        };
        const response = await this.sendRequest('POST', path, JSON.stringify(body));

        return await response.json();
    }

    /**
     * Used to restore password
     *
     * @param email - email of the user
     * @throws Error
     */
    public async forgotPassword(email: string): Promise<void> {
        const path = `/users/${email}/forgot-password`;

        return await this.sendRequest('GET', path, null);
    }

    /**
     * Used to change password
     *
     * @param password - old password of the user
     * @param newPassword - new password of the user
     * @throws Error
     */
    public async changePassword(password: string, newPassword: string): Promise<void> {
        const path = '/users/change-password';
        const body = {
            password: password,
            newPassword: newPassword,
        };

        return await this.sendRequest('POST', path, JSON.stringify(body));
    }

    /**
     * Used to delete account
     *
     * @param password - password of the user
     * @throws Error
     */
    public async delete(password: string): Promise<void> {
        const path = '/users/';
        const body = {
            password: password,
        };

        return await this.sendRequest('DELETE', path, JSON.stringify(body));
    }

    // TODO: remove secret after Vanguard release
    /**
     * Used to create account
     *
     * @param user - stores user information
     * @param secret - registration token used in Vanguard release
     * @param refUserId - referral id to participate in bonus program
     * @returns id of created user
     * @throws Error
     */
    public async create(user: User, password: string, secret: string, referrerUserId: string = ''): Promise<string> {
        const path = '/users/';
        const body = {
            secret: secret,
            referrerUserId: referrerUserId ? referrerUserId : '',
            password: password,
            fullName: user.fullName,
            shortName: user.shortName,
            email: user.email,
            partnerId: user.partnerId ? user.partnerId : '',
        };

        const response = await this.sendRequest('POST', path, JSON.stringify(body));
        const result = await response.json();

        return result.id;
    }
}
