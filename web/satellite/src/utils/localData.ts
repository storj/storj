// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * LocalData exposes methods to manage local storage.
 */
export class LocalData {
    private static userId = 'userId';
    private static selectedProjectId = 'selectedProjectId';
    private static userIdPassSalt = 'userIdPassSalt';
    private static serverSideEncryptionAcknowledge = 'serverSideEncryptionAcknowledge';
    private static demoBucketCreated = 'demoBucketCreated';

    public static getUserId(): string | null {
        return localStorage.getItem(LocalData.userId);
    }

    public static setUserId(id: string): void {
        localStorage.setItem(LocalData.userId, id);
    }

    public static removeUserId(): void {
        localStorage.removeItem(LocalData.userId);
    }

    public static getSelectedProjectId(): string | null {
        return localStorage.getItem(LocalData.selectedProjectId);
    }

    public static setSelectedProjectId(id: string): void {
        localStorage.setItem(LocalData.selectedProjectId, id);
    }

    public static removeSelectedProjectId(): void {
        localStorage.removeItem(LocalData.selectedProjectId);
    }

    public static getUserIDPassSalt(): UserIDPassSalt | null {
        const data: string | null = localStorage.getItem(LocalData.userIdPassSalt);
        if (data) {
            const parsed = JSON.parse(data);

            return new UserIDPassSalt(parsed.userId, parsed.passwordHash, parsed.salt);
        }

        return null;
    }

    public static setUserIDPassSalt(id: string, passwordHash: string, salt: string): void {
        const data = new UserIDPassSalt(id, passwordHash, salt);

        localStorage.setItem(LocalData.userIdPassSalt, JSON.stringify(data));
    }

    public static getDemoBucketCreatedStatus(): string | null {
        const status = localStorage.getItem(LocalData.demoBucketCreated);
        if (!status) return null;

        return JSON.parse(status)
    }

    public static setDemoBucketCreatedStatus(): void {
        localStorage.setItem(LocalData.demoBucketCreated, "true");
    }
}

/**
 * UserIDPassSalt is an entity holding user id, password hash and salt to be stored in local storage.
 */
export class UserIDPassSalt {
    public constructor(
        public userId: string = '',
        public passwordHash: string = '',
        public salt: string = '',
    ) {}
}
