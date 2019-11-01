// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { UpdatedUser, User } from '@/types/users';
import { HttpClient } from '@/utils/httpClient';

/**
 * AuthHttpApi is a console Auth API.
 * Exposes all auth-related functionality
 */
export class AuthHttpApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/auth';
    /**
     * Used to resend an registration confirmation email
     *
     * @param userId - id of newly created user
     * @throws Error
     */
    public async resendEmail(userId: string): Promise<void> {
        const path = `${this.ROOT_PATH}/resend-email/${userId}`;
        const response = await this.http.post(path, userId, false);
        if (response.ok) {
            return;
        }

        throw new Error('can not resend Email');
    }

    /**
     * Used to get authentication token
     *
     * @param email - email of the user
     * @param password - password of the user
     * @throws Error
     */
    public async token(email: string, password: string): Promise<string> {
        const path = `${this.ROOT_PATH}/token`;
        const body = {
            email: email,
            password: password,
        };
        const response = await this.http.post(path, JSON.stringify(body), false);
        if (response.ok) {
            return await response.json();
        }

        if (response.status === 500) {
            throw new Error('can not receive authentication token');
        }

        throw new Error('your email or password was incorrect, please try again');
    }

    /**
     * Used to restore password
     *
     * @param email - email of the user
     * @throws Error
     */
    public async forgotPassword(email: string): Promise<void> {
        const path = `${this.ROOT_PATH}/forgot-password/${email}`;
        const response = await this.http.post(path, email, false);
        if (response.ok) {
            return;
        }

        throw new Error('can not resend password');
    }

    /**
     * Used to update user full and short name
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
        const response = await this.http.patch(path, JSON.stringify(body), true);
        if (response.ok) {
            return;
        }

        throw new Error('can not update user data');
    }

    /**
     * Used to get user data
     *
     * @throws Error
     */
    public async get(): Promise<User> {
        const path = `${this.ROOT_PATH}/account`;
        const response = await this.http.get(path, true);
        if (response.ok) {
            return await response.json();
        }

        throw new Error('can not get user data');
    }

    /**
     * Used to change password
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
        const response = await this.http.post(path, JSON.stringify(body), true);
        if (response.ok) {
            return;
        }

        switch (response.status) {
            case 401: {
                throw new ErrorUnauthorized();
            }
            case 500: {
                throw new Error('can not change password');
            }
            default: {
                throw new Error('old password is incorrect, please try again');
            }
        }
    }

    /**
     * Used to delete account
     *
     * @param password - password of the user
     * @throws Error
     */
    public async delete(password: string): Promise<void> {
        const path = `${this.ROOT_PATH}/account/delete`;
        const body = {
            password: password,
        };
        const response = await this.http.post(path, JSON.stringify(body), true);
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
     * Used to register account
     *
     * @param user - stores user information
     * @param secret - registration token used in Vanguard release
     * @param referrerUserId - referral id to participate in bonus program
     * @returns id of created user
     * @throws Error
     */
    public async register(user: {fullName: string; shortName: string; email: string; partnerId: string; password: string}, secret: string, referrerUserId: string): Promise<string> {
        const path = `${this.ROOT_PATH}/register`;
        const body = {
            secret: secret,
            referrerUserId: referrerUserId ? referrerUserId : '',
            password: user.password,
            fullName: user.fullName,
            shortName: user.shortName,
            email: user.email,
            partnerId: user.partnerId ? user.partnerId : '',
        };

        const response = await this.http.post(path, JSON.stringify(body), false);
        if (!response.ok) {
            if (response.status === 500)
            {
                throw new Error('can not register user');
            }

            throw new Error('we are unable to create your account. This is an invite-only alpha, please join our waitlist to receive an invitation');
        }

        return await response.json();
    }
}
