// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { UpdatedUser, User, UsersApi } from '@/types/users';
import { BaseGql } from '@/api/baseGql';

/**
 * UsersApiGql is a graphql implementation of Users API.
 * Exposes all user-related functionality
 */
export class UsersApiGql extends BaseGql implements UsersApi {

    /**
     * Updates users full name and short name
     *
     * @param user - contains information that should be updated
     * @throws Error
     */
    public async update(user: UpdatedUser): Promise<void> {
        const query: string =
            `mutation ($fullName: String!, $shortName: String!) {
                updateAccount (
                    input: {
                        fullName: $fullName,
                        shortName: $shortName
                    }
                ) {
                    email,
                    fullName,
                    shortName
                }
            }`;

        const variables: any = {
            fullName: user.fullName,
            shortName: user.shortName,
        };

        await this.mutate(query, variables);
    }

    /**
     * Fetch user
     *
     * @returns User
     * @throws Error
     */
    public async get(): Promise<User> {
        const query =
            ` query {
                user {
                    id,
                    fullName,
                    shortName,
                    email,
                    partnerId,
                }
            }`;

        const response = await this.query(query);

        return this.fromJson(response.data.user);
    }

    private fromJson(user): User {
        return new User(user.id, user.fullName, user.shortName, user.email, user.partnerId);
    }
}
