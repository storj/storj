// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { User } from '@/types/users';

/**
 * AuthApiGql is a graphql implementation of Auth API.
 * Exposes all auth-related functionality
 */
export class AuthApi extends BaseGql {
    /**
     * Used to resend an registration confirmation email
     *
     * @param userId - id of newly created user
     * @throws Error
     */
    public async resendEmail(userId: string): Promise<void> {
        const query =
            `query ($userId: String!){
                resendAccountActivationEmail(id: $userId)
            }`;

        const variables = {
            userId,
        };

        await this.query(query, variables);
    }

    /**
     * Used to get authentication token
     *
     * @param email - email of the user
     * @param password - password of the user
     * @throws Error
     */
    public async token(email: string, password: string): Promise<string> {
        const query =
            ` query ($email: String!, $password: String!) {
                token(email: $email, password: $password) {
                    token
                }
            }`;

        const variables = {
            email,
            password,
        };

        const response = await this.query(query, variables);

        return response.data.token.token;
    }

    /**
     * Used to restore password
     *
     * @param email - email of the user
     * @throws Error
     */
    public async forgotPassword(email: string): Promise<void> {
        const query =
            `query($email: String!) {
                forgotPassword(email: $email)
            }`;

        const variables = {
            email,
        };

        await this.query(query, variables);
    }

    /**
     * Used to change password
     *
     * @param password - old password of the user
     * @param newPassword - new password of the user
     * @throws Error
     */
    public async changePassword(password: string, newPassword: string): Promise<void> {
        const query =
            `mutation($password: String!, $newPassword: String!) {
                changePassword (
                    password: $password,
                    newPassword: $newPassword
                ) {
                   email
                }
            }`;

        const variables = {
            password,
            newPassword,
        };

        await this.mutate(query, variables);
    }

    /**
     * Used to delete account
     *
     * @param password - password of the user
     * @throws Error
     */
    public async delete(password: string): Promise<void> {
        const query =
            `mutation ($password: String!){
                deleteAccount(password: $password) {
                    email
                }
            }`;

        const variables = {
            password,
        };

        await this.mutate(query, variables);
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
        const query =
            `mutation($email: String!, $password: String!, $fullName: String!, $shortName: String!,
                     $partnerID: String!, $referrerUserId: String!, $secret: String!) {
                createUser(
                    input: {
                        email: $email,
                        password: $password,
                        fullName: $fullName,
                        shortName: $shortName,
                        partnerId: $partnerID
                    },
                    referrerUserId: $referrerUserId,
                    secret: $secret,
                ) {email, id}
            }`;

        const variables = {
            email: user.email,
            fullName: user.fullName,
            shortName: user.shortName,
            partnerID: user.partnerId ? user.partnerId : '',
            referrerUserId: referrerUserId ? referrerUserId : '',
            password,
            secret,
        };

        const response = await this.mutate(query, variables);

        return response.data.createUser.id;
    }
}
