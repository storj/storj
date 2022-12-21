// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { UpdatedUser, User, UsersApi } from '@/types/users';

/**
 * Mock for UsersApi
 */
export class UsersApiMock implements UsersApi {
    private mockUser: User;

    public setMockUser(mockUser: User): void {
        this.mockUser = mockUser;
    }

    public get(): Promise<User> {
        return Promise.resolve(this.mockUser);
    }

    public getFrozenStatus(): Promise<boolean> {
        return Promise.resolve(true);
    }

    public update(_user: UpdatedUser): Promise<void> {
        throw new Error('not implemented');
    }

    public enableUserMFA(_: string): Promise<void> {
        return Promise.resolve();
    }

    public disableUserMFA(_passcode: string, _recoveryCode: string): Promise<void> {
        return Promise.resolve();
    }

    public generateUserMFASecret(): Promise<string> {
        return Promise.resolve('test');
    }

    public generateUserMFARecoveryCodes(): Promise<string[]> {
        return Promise.resolve(['test', 'test1', 'test2']);
    }
}
