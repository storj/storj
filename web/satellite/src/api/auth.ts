// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorEmailUsed } from '@/api/errors/ErrorEmailUsed';
import { ErrorTooManyRequests } from '@/api/errors/ErrorTooManyRequests';
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
     * Used to resend an registration confirmation email.
     *
     * @param userId - id of newly created user
     * @throws Error
     */
    public async resendEmail(userId: string): Promise<void> {
        const path = `${this.ROOT_PATH}/resend-email/${userId}`;
        const response = await this.http.post(path, userId);
        if (response.ok) {
            return;
        }

        throw new Error('can not resend Email');
    }

    /**
     * Used to get authentication token.
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
        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return await response.json();
        }

        switch (response.status) {
            case 401:
                throw new ErrorUnauthorized('Your email or password was incorrect, please try again');
            case 429:
                throw new ErrorTooManyRequests('You\'ve exceeded limit of attempts, try again in 5 minutes');
            default:
                throw new Error('Can not receive authentication token');
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

        throw new Error('There is no such email');
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
     * @returns id of created user
     * @throws Error
     */
    public async register(user: {fullName: string; shortName: string; email: string; partner: string; partnerId: string; password: string; isProfessional: boolean; position: string; companyName: string; employeeCount: string}, secret: string): Promise<string> {
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
        };

        const response = await this.http.post(path, JSON.stringify(body));
        if (!response.ok) {
            switch (response.status) {
                case 401:
                    throw new ErrorUnauthorized('We are unable to create your account. This is an invite-only alpha, please join our waitlist to receive an invitation');
                case 409:
                    throw new ErrorEmailUsed('This email is already in use, try another');
                case 429:
                    throw new ErrorTooManyRequests('You\'ve exceeded limit of attempts, try again in 5 minutes');
                default:
                    throw new Error('Can not register user');
            }
        }

        return await response.json();
    }
}
