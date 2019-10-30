// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all user-related functionality
 */
export interface UsersApi {
    /**
     * Updates users full name and short name
     *
     * @param user - contains information that should be updated
     * @throws Error
     */
    update(user: UpdatedUser): Promise<void>;
    /**
     * Fetch user
     *
     * @returns User
     * @throws Error
     */
    get(): Promise<User>;
}

/**
 * User class holds info for User entity.
 */
export class User {
    public id: string;
    public fullName: string;
    public shortName: string;
    public email: string;
    public partnerId: string;
    public password: string;

    public constructor(id: string = '', fullName: string = '', shortName: string = '', email: string = '', partnerId: string = '', password: string = '') {
        this.id = id;
        this.fullName = fullName;
        this.shortName = shortName;
        this.email = email;
        this.partnerId = partnerId;
        this.password = password;
    }

    public getFullName(): string {
        return this.shortName === '' ? this.fullName : this.shortName;
    }
}

/**
 * User class holds info for updating User.
 */
export class UpdatedUser {
    public fullName: string;
    public shortName: string;

    public constructor(fullName: string, shortName: string) {
        this.fullName = fullName;
        this.shortName = shortName;
    }

    public setFullName(value: string) {
        this.fullName = value.trim();
    }

    public setShortName(value: string) {
        this.shortName = value.trim();
    }

    public isValid(): boolean {
        return !!this.fullName;
    }
}
