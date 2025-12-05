// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * LocalData exposes methods to manage local storage.
 */
export class LocalData {
    private static bucketWasCreated = 'bucketWasCreated';
    private static sessionExpirationDate = 'sessionExpirationDate';
    private static customSessionDuration = 'customSessionDuration';
    private static projectTableViewEnabled = 'projectTableViewEnabled';
    private static browserCardViewEnabled = 'browserCardViewEnabled';
    private static sessionHasExpired = 'sessionHasExpired';
    private static objectCountOfSelectedBucket = 'objectCountOfSelectedBucket';

    public static setObjectCountOfSelectedBucket(count: number): void {
        localStorage.setItem(LocalData.objectCountOfSelectedBucket, count.toString());
    }

    public static getObjectCountOfSelectedBucket(): number | null {
        const count = localStorage.getItem(LocalData.objectCountOfSelectedBucket);
        return count ? parseInt(count) : null;
    }

    public static setBucketWasCreatedStatus(): void {
        localStorage.setItem(LocalData.bucketWasCreated, 'true');
    }

    public static getBucketWasCreatedStatus(): boolean | null {
        const status = localStorage.getItem(LocalData.bucketWasCreated);
        if (!status) return null;

        return JSON.parse(status);
    }

    public static getSessionExpirationDate(): Date | null {
        const data: string | null = localStorage.getItem(LocalData.sessionExpirationDate);
        if (data) {
            return new Date(data);
        }

        return null;
    }

    public static setSessionExpirationDate(date: Date): void {
        localStorage.setItem(LocalData.sessionExpirationDate, date.toISOString());
    }

    public static getCustomSessionDuration(): number | null {
        const value: string | null = localStorage.getItem(LocalData.customSessionDuration);
        if (value) return parseInt(value);

        return null;
    }

    public static setCustomSessionDuration(value: number): void {
        localStorage.setItem(LocalData.customSessionDuration, value.toString());
    }

    public static removeCustomSessionDuration(): void {
        localStorage.removeItem(LocalData.customSessionDuration);
    }

    public static getSessionHasExpired(): boolean {
        const value: string | null = localStorage.getItem(LocalData.sessionHasExpired);
        return value === 'true';
    }

    public static setSessionHasExpired(): void {
        localStorage.setItem(LocalData.sessionHasExpired, 'true');
    }

    public static removeSessionHasExpired(): void {
        localStorage.removeItem(LocalData.sessionHasExpired);
    }

    public static getProjectTableViewEnabled(): boolean {
        const value = localStorage.getItem(LocalData.projectTableViewEnabled);
        return value === 'true';
    }

    public static setProjectTableViewEnabled(enabled: boolean): void {
        localStorage.setItem(LocalData.projectTableViewEnabled, enabled.toString());
    }

    /*
    * Whether the file browser should use the card view.
    */
    public static getBrowserCardViewEnabled(): boolean {
        const value = localStorage.getItem(LocalData.browserCardViewEnabled);
        return value === 'true';
    }

    /*
    * Set whether the file browser should use the card view.
    */
    public static setBrowserCardViewEnabled(enabled: boolean): void {
        localStorage.setItem(LocalData.browserCardViewEnabled, enabled.toString());
    }

    /*
    * Whether a user defined setting has been made for the projects table
    * */
    public static hasProjectTableViewConfigured(): boolean {
        return localStorage.getItem(LocalData.projectTableViewEnabled) !== null;
    }

    /*
    * Remove the user defined setting for the projects table;
    * */
    public static removeProjectTableViewConfig() {
        return localStorage.removeItem(LocalData.projectTableViewEnabled);
    }
}
