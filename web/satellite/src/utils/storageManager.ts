// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class StorageManager {

    public static setIsReferralNotificationHidden(): void {
        StorageManager.setItem('isReferralNotificationHidden', 'true');
    }

    public static get isReferralNotificationHidden(): boolean {
        return !!StorageManager.getItem('isReferralNotificationHidden');
    }

    public static setItem(key: string, value: string): void {
        localStorage.setItem(key, value);
    }

    private static getItem(key: string) {
        return localStorage.getItem(key);
    }
}
